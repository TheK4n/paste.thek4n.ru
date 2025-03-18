package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Record struct {
	Body       []byte `redis:"body"`
	Disposable bool   `redis:"disposable"`
}

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
	exists, err := db.Exists(ctx, key)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, ErrKeyNotFound
	}

	var record Record
	err = db.client.HGetAll(ctx, key).Scan(&record)

	if err != nil {
		return nil, err
	}

	if record.Disposable {
		delErr := db.client.Del(ctx, key).Err()
		if delErr != nil {
			panic("Fatal error when deletion disposable url: " + delErr.Error())
		}
	}

	return record.Body, nil
}

func (db *RedisDB) Set(ctx context.Context, key string, body []byte, ttl time.Duration) error {
	return db.set(ctx, key, body, ttl, false)
}

func (db *RedisDB) SetDisposable(ctx context.Context, key string, body []byte, ttl time.Duration) error {
	return db.set(ctx, key, body, ttl, true)
}

func (db *RedisDB) set(ctx context.Context, key string, body []byte, ttl time.Duration, disposable bool) error {
	record := Record{
		Body:       body,
		Disposable: disposable,
	}

	err := db.client.HSet(ctx, key, record).Err()
	if err != nil {
		return err
	}

	return db.client.Expire(ctx, key, ttl).Err()
}

func (db *RedisDB) Ping(ctx context.Context) bool {
	if err := db.client.Ping(ctx).Err(); err != nil {
		return false
	}

	return true
}

func InitStorageDB(dbHost string, dbPort int) (*RedisDB, error) {
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
