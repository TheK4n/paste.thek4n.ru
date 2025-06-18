//go:build integration

package storage

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thek4n/paste.thek4n.name/internal/config"
)

func setupTestKeysDB(t *testing.T) *KeysDB {
	dbHost := os.Getenv("REDIS_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := 6379

	db, err := InitKeysStorageDB(dbHost, dbPort)
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Очищаем базу перед тестом
	err = db.Client.FlushDB(context.Background()).Err()
	assert.NoError(t, err)

	return db
}

func TestKeysDB_Exists(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()
	key := "test_key"

	t.Run("key does not exist", func(t *testing.T) {
		exists, err := db.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("key exists", func(t *testing.T) {
		err := db.Client.Set(ctx, key, "value", 0).Err()
		assert.NoError(t, err)

		exists, err := db.Exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestKeysDB_Get(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()
	key := "test_key"
	testBody := []byte("test body")

	t.Run("key not found", func(t *testing.T) {
		_, err := db.Get(ctx, "nonexistent_key")
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("get non-disposable key", func(t *testing.T) {
		record := KeyRecord{
			URL:    true,
			Body:   testBody,
			Clicks: 0,
		}
		err := db.Set(ctx, key, 0, record)
		assert.NoError(t, err)

		result, err := db.Get(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, testBody, result.Body)
		assert.True(t, result.URL)

		// Проверяем что счетчик кликов увеличился
		clicks, err := db.GetClicks(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, 1, clicks)
	})

	t.Run("get disposable key", func(t *testing.T) {
		disposableKey := "disposable_key"
		record := KeyRecord{
			Disposable: true,
			Countdown:  1, // Одно использование
			Body:       testBody,
		}
		err := db.Set(ctx, disposableKey, 0, record)
		assert.NoError(t, err)

		// Первое использование - должно вернуть данные и удалить ключ
		result, err := db.Get(ctx, disposableKey)
		assert.NoError(t, err)
		assert.Equal(t, testBody, result.Body)

		// Проверяем что ключ удален
		exists, err := db.Exists(ctx, disposableKey)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("get compressed data", func(t *testing.T) {
		compressedKey := "compressed_key"
		largeBody := bytes.Repeat([]byte("a"), config.COMPRESS_THRESHOLD_BYTES+1)

		record := KeyRecord{
			Body: largeBody,
		}
		err := db.Set(ctx, compressedKey, 0, record)
		assert.NoError(t, err)

		result, err := db.Get(ctx, compressedKey)
		assert.NoError(t, err)
		assert.Equal(t, largeBody, result.Body)
	})
}

func TestKeysDB_GetClicks(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()
	key := "test_key"

	t.Run("key not found", func(t *testing.T) {
		_, err := db.GetClicks(ctx, "nonexistent_key")
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("get clicks count", func(t *testing.T) {
		record := KeyRecord{
			Body:   []byte("test"),
			Clicks: 5,
		}
		err := db.Set(ctx, key, 0, record)
		assert.NoError(t, err)

		clicks, err := db.GetClicks(ctx, key)
		assert.NoError(t, err)
		assert.Equal(t, 5, clicks)
	})
}

func TestKeysDB_Set(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()
	key := "test_key"
	testBody := []byte("test body")

	t.Run("set simple record", func(t *testing.T) {
		record := KeyRecord{
			URL:  true,
			Body: testBody,
		}
		err := db.Set(ctx, key, 0, record)
		assert.NoError(t, err)

		var result KeyRecord
		err = db.Client.HGetAll(ctx, key).Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, testBody, result.Body)
		assert.True(t, result.URL)
	})

	t.Run("set with TTL", func(t *testing.T) {
		ttl := 10 * time.Second
		record := KeyRecord{
			Body: testBody,
		}
		err := db.Set(ctx, "ttl_key", ttl, record)
		assert.NoError(t, err)

		actualTTL := db.Client.TTL(ctx, "ttl_key").Val()
		assert.True(t, actualTTL > 0 && actualTTL <= ttl)
	})

	t.Run("auto compress large data", func(t *testing.T) {
		largeBody := bytes.Repeat([]byte("a"), config.COMPRESS_THRESHOLD_BYTES+1)
		record := KeyRecord{
			Body: largeBody,
		}
		err := db.Set(ctx, "large_key", 0, record)
		assert.NoError(t, err)

		var result KeyRecord
		err = db.Client.HGetAll(ctx, "large_key").Scan(&result)
		assert.NoError(t, err)
		assert.True(t, isCompressed(result.Body))
	})
}

func TestInitKeysStorageDB(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		dbHost := os.Getenv("REDIS_HOST")
		if dbHost == "" {
			dbHost = "localhost"
		}
		db, err := InitKeysStorageDB(dbHost, 6379)
		assert.NoError(t, err)
		assert.NotNil(t, db.Client)
	})

	t.Run("connection error", func(t *testing.T) {
		db, err := InitKeysStorageDB("invalid_host", 6379)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func TestCompression(t *testing.T) {
	t.Run("compress and decompress", func(t *testing.T) {
		data := []byte("test data to compress")
		compressed, err := compress(data)
		assert.NoError(t, err)
		assert.True(t, isCompressed(compressed))

		decompressed, err := decompress(compressed)
		assert.NoError(t, err)
		assert.Equal(t, data, decompressed)
	})

	t.Run("decompress invalid data", func(t *testing.T) {
		_, err := decompress([]byte("invalid gzip data"))
		assert.Error(t, err)
	})
}
