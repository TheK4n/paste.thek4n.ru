package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thek4n/paste.thek4n.name/internal/config"
)

var (
	ErrKeyNotFound = errors.New("key not found in db")
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

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

// Get returns KeyRecordAnswer with body by key
// increases clicks for key and removes if disposable counter exhausted
// decompresses body if it compressed in db
func (db *KeysDB) Get(ctx context.Context, key string) (KeyRecordAnswer, error) {
	var answer KeyRecordAnswer
	var record KeyRecord
	err := db.Client.HGetAll(ctx, key).Scan(&record)
	if err != nil {
		return answer, err
	}

	if record.Body == nil {
		return answer, ErrKeyNotFound
	}

	clicks, clicksErr := db.Client.HIncrBy(ctx, key, "clicks", 1).Result()
	if clicksErr != nil {
		return answer, clicksErr
	}
	log.Printf("Increased click counter for key '%s', now: %d", key, clicks)

	if record.Disposable {
		countdown, countdownErr := db.Client.HIncrBy(ctx, key, "countdown", -1).Result()
		if countdownErr != nil {
			return answer, fmt.Errorf("fatal error when countdown disposable counter: %w", countdownErr)
		}
		log.Printf("Decreased countdown disposable key '%s' when getting, countdown=%d", key, countdown)

		if countdown < 1 {
			delErr := db.Client.Del(ctx, key).Err()
			if delErr != nil {
				return answer, fmt.Errorf("fatal error when delete disposable url: %w", delErr)
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
	clicks, err := db.Client.HGet(ctx, key, "clicks").Result()
	if err == redis.Nil {
		return 0, ErrKeyNotFound
	}

	if err != nil {
		return 0, err
	}

	return strconv.Atoi(clicks)
}

// Set KeyRecord in db by key
// compresses if body size bigger then threshold config value
// if ttl equals zero key has endless time to live
func (db *KeysDB) Set(ctx context.Context, key string, ttl time.Duration, record KeyRecord) error {
	if len(record.Body) > config.COMPRESS_THRESHOLD_BYTES {
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
		PoolSize:     100,
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
