package keys

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

func setupTestKeysDB(t *testing.T) *storage.KeysDB {
	dbHost := os.Getenv("REDIS_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := 6379

	db, err := storage.InitKeysStorageDB(dbHost, dbPort)
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Очищаем базу перед тестом
	err = db.Client.FlushDB(context.Background()).Err()
	assert.NoError(t, err)

	return db
}

func TestGet(t *testing.T) {
	db := *setupTestKeysDB(t)
	ctx := context.Background()
	key := "test_key"
	timeout := 1 * time.Second
	testBody := []byte("test body")

	t.Run("successful get", func(t *testing.T) {
		// Подготавливаем тестовые данные
		err := db.Set(ctx, key, 0, storage.KeyRecord{Body: testBody})
		assert.NoError(t, err)

		result, err := Get(db, key, timeout)
		assert.NoError(t, err)
		assert.Equal(t, testBody, result.Body)
	})

	t.Run("key not found", func(t *testing.T) {
		_, err := Get(db, "nonexistent_key", timeout)
		assert.Error(t, err)
		assert.Equal(t, storage.ErrKeyNotFound, err)
	})
}

func TestGetClicks(t *testing.T) {
	db := *setupTestKeysDB(t)
	ctx := context.Background()
	key := "test_key"
	timeout := 1 * time.Second

	t.Run("successful get clicks", func(t *testing.T) {
		// Подготавливаем тестовые данные
		err := db.Set(ctx, key, 0, storage.KeyRecord{Clicks: 5})
		assert.NoError(t, err)

		clicks, err := GetClicks(db, key, timeout)
		assert.NoError(t, err)
		assert.Equal(t, 5, clicks)
	})

	t.Run("key not found", func(t *testing.T) {
		_, err := GetClicks(db, "nonexistent_key", timeout)
		assert.Error(t, err)
		assert.Equal(t, storage.ErrKeyNotFound, err)
	})
}

func TestCache(t *testing.T) {
	db := *setupTestKeysDB(t)
	ctx := context.Background()
	timeout := 1 * time.Second
	ttl := 24 * time.Hour
	record := storage.KeyRecord{Body: []byte("test")}

	t.Run("successful cache with requested key", func(t *testing.T) {
		requestedKey := "custom_key"
		result, err := Cache(db, timeout, requestedKey, ttl, 10, record)
		assert.NoError(t, err)
		assert.Equal(t, requestedKey, result)

		// Проверяем что данные действительно записались
		answer, err := Get(db, requestedKey, timeout)
		assert.NoError(t, err)
		assert.Equal(t, record.Body, answer.Body)
	})

	t.Run("requested key already taken", func(t *testing.T) {
		requestedKey := "taken_key"
		// Сначала создаем ключ
		err := db.Set(ctx, requestedKey, 0, record)
		assert.NoError(t, err)

		_, err = Cache(db, timeout, requestedKey, ttl, 10, record)
		assert.Error(t, err)
		assert.Equal(t, ErrKeyAlreadyTaken, err)
	})

	t.Run("successful cache with generated key", func(t *testing.T) {
		result, err := Cache(db, timeout, "", ttl, 10, record)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
		assert.Len(t, result, 10)

		// Проверяем что данные действительно записались
		answer, err := db.Get(ctx, result)
		assert.NoError(t, err)
		assert.Equal(t, record.Body, answer.Body)
	})
}
