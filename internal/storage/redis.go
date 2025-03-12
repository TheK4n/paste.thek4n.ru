package storage

import (
	"context"
	"fmt"
	"time"
	"log"

	"github.com/redis/go-redis/v9"
)

const DEFAULT_REDIS_PORT = 6379

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

func (db *RedisDB) Ping(ctx context.Context) bool {
	if err := db.client.Ping(ctx).Err(); err != nil {
		return false
	}

	return true
}

func InitStorageDB(dbHost string, dbPort int) (*RedisDB, error) {
	if dbPort == 0 {
		dbPort = DEFAULT_REDIS_PORT
	}
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

	log.Printf("Connected to database on %s:%d\n", dbHost, dbPort)

	return &RedisDB{client: client}, nil
}
