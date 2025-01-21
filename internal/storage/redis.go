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

func (db *RedisDB) Exists(key string) (bool, error) {
	keysNumber, err := db.client.Exists(context.Background(), key).Uint64()
	if err != nil {
		return false, err
	}

	return keysNumber > 0, nil
}

// Get key from redis db. It returns nil, nil if key not exists
func (db *RedisDB) Get(key string) ([]byte, error) {
	result := db.client.Get(context.Background(), key)

	if result.Err() == redis.Nil {
		return nil, nil
	}

	if result.Err() != nil {
		return nil, result.Err()
	}

	return result.Bytes()
}

func (db *RedisDB) Set(key string, body []byte) error {
	return db.client.Set(context.Background(), key, body, 0).Err()
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
