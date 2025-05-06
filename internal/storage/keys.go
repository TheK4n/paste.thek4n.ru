package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const COMPRESS_THRESHOLD = 2048

var (
	ErrKeyNotFound = errors.New("Key not found in db")
)

type KeyRecord struct {
	Disposable bool   `redis:"disposable"`
	URL        bool   `redis:"url"`
	Countdown  int    `redis:"countdown"`
	Clicks     int    `redis:"clicks"`
	Body       []byte `redis:"body"`
}

type KeyRecordAnswer struct {
	URL  bool   `redis:"url"`
	Body []byte `redis:"body"`
}

type KeysDB struct {
	Client *redis.Client
}

func (db *KeysDB) Exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := db.Client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, err
	}

	return keysNumber > 0, nil
}

func (db *KeysDB) Get(ctx context.Context, key string) (KeyRecordAnswer, error) {
	var answer KeyRecordAnswer
	exists, err := db.Exists(ctx, key)

	if err != nil {
		return answer, err
	}

	if !exists {
		return answer, ErrKeyNotFound
	}

	var record KeyRecord
	err = db.Client.HGetAll(ctx, key).Scan(&record)
	if err != nil {
		return answer, err
	}

	clicks, clicksErr := db.Client.HIncrBy(ctx, key, "clicks", 1).Result()
	if clicksErr != nil {
		return answer, clicksErr
	}
	log.Printf("Increased click counter for key '%s', now: %d", key, clicks)

	if record.Disposable {
		countdown, countdownErr := db.Client.HIncrBy(ctx, key, "countdown", -1).Result()
		if countdownErr != nil {
			panic("Fatal error when countdown: " + countdownErr.Error())
		}
		log.Printf("Decreased countdown disposable key '%s' when getting, countdown=%d", key, countdown)

		if countdown < 1 {
			delErr := db.Client.Del(ctx, key).Err()
			if delErr != nil {
				panic("Fatal error when deletion disposable url: " + delErr.Error())
			}
			log.Printf("Removed disposable key '%s' when getting", key)
		}
	}

	if isCompressed(record.Body) {
		decompressedBody, err := decompress(record.Body)
		if err != nil {
			return answer, err
		}

		record.Body = decompressedBody
	}

	answer.Body = record.Body
	answer.URL = record.URL

	return answer, nil
}

func (db *KeysDB) GetClicks(ctx context.Context, key string) (int, error) {
	exists, err := db.Exists(ctx, key)
	if err != nil {
		return 0, err
	}

	if !exists {
		return 0, ErrKeyNotFound
	}

	clicks, err := db.Client.HGet(ctx, key, "clicks").Result()
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(clicks)
}

func (db *KeysDB) Set(ctx context.Context, key string, ttl time.Duration, record KeyRecord) error {
	if len(record.Body) > COMPRESS_THRESHOLD {
		compressedBody, err := compress(record.Body)
		if err != nil {
			return fmt.Errorf("failed to compress: %w", err)
		}

		record.Body = compressedBody
	}

	err := db.Client.HSet(ctx, key, record).Err()
	if err != nil {
		return err
	}

	if ttl != time.Duration(0) {
		return db.Client.Expire(ctx, key, ttl).Err()
	}

	return nil
}

func (db *KeysDB) Ping(ctx context.Context) bool {
	return db.Client.Ping(ctx).Err() == nil
}

func InitKeysStorageDB(dbHost string, dbPort int) (*KeysDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", dbHost, dbPort),
		Password:     "",
		Username:     "",
		DB:           0,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	log.Printf("Connected to database 0 (keys) on %s:%d\n", dbHost, dbPort)

	return &KeysDB{Client: client}, nil
}

// compress сжимает данные из слайса байт с использованием gzip по чанкам
func compress(data []byte) ([]byte, error) {
	chunkSize := 2048
	buf := bytes.NewBuffer(make([]byte, 0, len(data)))

	gz, err := gzip.NewWriterLevel(buf, gzip.DefaultCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gz.Close()

	for offset := 0; offset < len(data); offset += chunkSize {
		end := min(offset+chunkSize, len(data))

		if _, err := gz.Write(data[offset:end]); err != nil {
			return nil, fmt.Errorf("failed to write chunk: %w", err)
		}

		if err := gz.Flush(); err != nil {
			return nil, fmt.Errorf("failed to flush: %w", err)
		}
	}

	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	uncompressed, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return uncompressed, nil
}

func isCompressed(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}
