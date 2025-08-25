package repository

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type redisKeyRecord struct {
	Body      []byte        `redis:"body"`
	TTL       time.Duration `redis:"ttl"`
	Clicks    uint32        `redis:"clicks"`
	Countdown uint8         `redis:"countdown"`
	Eternal   bool          `redis:"eternal"`
	URL       bool          `redis:"url"`
}

// RedisRecordRepository redis implementation of domain interface.
type RedisRecordRepository struct {
	client *redis.Client
	config config.CachingConfig
}

// NewRedisRecordRepository constructor.
func NewRedisRecordRepository(c *redis.Client, cfg config.CachingConfig) *RedisRecordRepository {
	return &RedisRecordRepository{
		client: c,
		config: cfg,
	}
}

// GetByKey fetch Record from redis db.
func (r *RedisRecordRepository) GetByKey(ctx context.Context, key objectvalue.RecordKey) (aggregate.Record, error) {
	exists, err := r.exists(ctx, key)
	if err != nil {
		return aggregate.Record{}, fmt.Errorf("fail to check record existing by key '%s': %w", key, err)
	}
	if !exists {
		return aggregate.Record{}, domainerrors.ErrRecordNotFound
	}

	var record redisKeyRecord
	err = r.client.HGetAll(ctx, string(key)).Scan(&record)
	if err != nil {
		return aggregate.Record{}, fmt.Errorf("fail to get record by key '%s': %w", key, err)
	}

	if isCompressed(record.Body) {
		decompressedBody, err := decompress(record.Body, r.config.MaxBodySize())
		if err != nil {
			return aggregate.Record{}, fmt.Errorf("fail to decompress compressed body: %w", err)
		}

		record.Body = decompressedBody
	}

	res, err := aggregate.NewRecord(
		string(key),
		time.Time(objectvalue.NewExpirationDateFromTTL(record.TTL)),
		record.Countdown,
		record.Eternal,
		record.Clicks,
		record.Body,
		record.URL,
	)
	if err != nil {
		return res, fmt.Errorf("fail to create new record: %w", err)
	}

	return res, nil
}

// SetByKey writes Record to redis db.
func (r *RedisRecordRepository) SetByKey(ctx context.Context, key objectvalue.RecordKey, record aggregate.Record) error {
	recordDisposableCounter := record.DisposableCounter()
	if recordDisposableCounter < 0 || recordDisposableCounter > math.MaxUint8 {
		return fmt.Errorf("disposable counter bigger then %d or less then 0", math.MaxUint8)
	}

	rec := &redisKeyRecord{
		URL:       record.URL(),
		Clicks:    record.Clicks(),
		Countdown: uint8(recordDisposableCounter),
		Eternal:   record.DisposableCounterEternal(),
		TTL:       record.TTL(),
		Body:      record.RGetBody(),
	}

	if len(rec.Body) > int(r.config.CompressThresholdBytes()) {
		compressedBody, err := compress(rec.Body)
		if err != nil {
			return fmt.Errorf("failed to compress: %w", err)
		}

		rec.Body = compressedBody
	}

	err := r.client.HSet(ctx, string(key), rec).Err()
	if err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	ttl := record.TTL()
	if ttl != time.Duration(0) {
		err := r.client.Expire(ctx, string(key), ttl).Err()
		if err != nil {
			return fmt.Errorf("failed to set expire for key '%s': %w", key, err)
		}
	}

	return nil
}

// Exists returns is record with this key exists.
func (r *RedisRecordRepository) Exists(ctx context.Context, key objectvalue.RecordKey) (bool, error) {
	return r.exists(ctx, key)
}

// Exists returns is record with this key exists.
func (r *RedisRecordRepository) exists(ctx context.Context, key objectvalue.RecordKey) (bool, error) {
	keysNumber, err := r.client.Exists(ctx, string(key)).Uint64()
	if err != nil {
		return false, fmt.Errorf("fail to check exists for key '%s': %w", key, err)
	}

	return keysNumber > 0, nil
}

// GenerateUniqueKey Generates unique key with minimum length of minLength
// using charset. Increases minLength if was attemptsToIncreaseMinLength
// attempts generate unique key. Returns error if database error or context
// done or maxLength reached.
func (r *RedisRecordRepository) GenerateUniqueKey(
	ctx context.Context,
	minLength, maxLength uint8,
) (objectvalue.RecordKey, error) {
	var err error
	var key string
	currentAttemptsCountdown := r.config.AttemptsToIncreaseKeyMinLength()

	// initial true for start cycle
	exists := true
	for exists {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout")
		default:
		}

		key, err = generateKey(minLength, r.config.KeysCharset())
		if err != nil {
			return "", fmt.Errorf("fail generate key: %w", err)
		}
		exists, err = r.Exists(ctx, objectvalue.RecordKey(key))
		if err != nil {
			return "", fmt.Errorf("fail to check is key '%s' exists: %w", key, err)
		}
		currentAttemptsCountdown--

		if currentAttemptsCountdown < 1 {
			minLength++
			currentAttemptsCountdown = r.config.AttemptsToIncreaseKeyMinLength()
		}

		if minLength > maxLength {
			return "", fmt.Errorf("max key length reached")
		}
	}

	return objectvalue.RecordKey(key), nil
}

// generateKey generate new random key with specified length using charset.
func generateKey(length uint8, charset string) (string, error) {
	result := make([]byte, length)
	charsetLen := len(charset)

	for i := range length {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(charsetLen)))
		if err != nil {
			return "", fmt.Errorf("failure generate random number: %w", err)
		}
		result[i] = charset[nBig.Int64()]
	}

	return string(result), nil
}

func compress(data []byte) ([]byte, error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buf)
	buf.Reset()

	buf.Grow(len(data) / 2)

	gz, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	if _, err := gz.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write data: %w", err)
	}

	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func decompress(data []byte, limit int64) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buf)
	buf.Reset()
	buf.Grow(len(data) * 2)

	_, err = io.CopyN(buf, gz, limit)
	if !errors.Is(err, io.EOF) && err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip reader: %w", err)
	}

	return buf.Bytes(), nil
}

func isCompressed(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}
