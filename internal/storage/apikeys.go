// Package storage provides api for getting keys from db.
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// APIKeyRecord contains redis data.
type APIKeyRecord struct {
	ID    string `redis:"id"`
	Valid bool   `redis:"valid"`
}

// APIKeysDB contains db connection.
type APIKeysDB struct {
	Client *redis.Client
}

// Set saves key in db.
func (db *APIKeysDB) Set(ctx context.Context, key string) error {
	record := APIKeyRecord{
		Valid: true,
	}

	err := db.Client.HSet(ctx, key, record).Err()
	if err != nil {
		return fmt.Errorf("fail to set record for key '%s': %w", key, err)
	}

	return nil
}

// Get gets key from db.
// returns APIKeyRecord if found.
// if key not found returns ErrKeyNotFound.
func (db *APIKeysDB) Get(ctx context.Context, key string) (APIKeyRecord, error) {
	var record APIKeyRecord
	exists, err := db.exists(ctx, key)
	if err != nil {
		return record, err
	}

	if !exists {
		return record, ErrKeyNotFound
	}

	err = db.Client.HGetAll(ctx, key).Scan(&record)
	if err != nil {
		return record, fmt.Errorf("fail to get record for key '%s': %w", key, err)
	}

	return record, nil
}

func (db *APIKeysDB) exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := db.Client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, fmt.Errorf("fait to check key existing: %w", err)
	}

	return keysNumber > 0, nil
}

// InitAPIKeysStorageDB returns valid APIKeysDB.
func InitAPIKeysStorageDB(dbHost string, dbPort int) (*APIKeysDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", dbHost, dbPort),
		Password:     "",
		Username:     "",
		DB:           1,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("fail to check connection: %w", err)
	}

	return &APIKeysDB{Client: client}, nil
}
