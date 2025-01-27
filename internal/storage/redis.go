package storage

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	client *redis.Client
}

func (db *RedisDB) Exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := db.client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, err
	}

	return keysNumber > 0, nil
}

func (db *RedisDB) Get(ctx context.Context, key string) ([]byte, error) {
	result := db.client.Get(ctx, key)

	if result.Err() == redis.Nil {
		return nil, ErrKeyNotFound
	}

	if result.Err() != nil {
		return nil, result.Err()
	}

	return result.Bytes()
}

func (db *RedisDB) Set(ctx context.Context, key string, body []byte, ttl time.Duration) error {
	return db.client.Set(ctx, key, body, ttl).Err()
}

func InitStorageDB() (*RedisDB, error) {
	redisHost := os.Getenv("REDIS_HOST")

	client := redis.NewClient(&redis.Options{
		Addr:         redisHost + ":6379",
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

	return &RedisDB{client: client}, nil
}
