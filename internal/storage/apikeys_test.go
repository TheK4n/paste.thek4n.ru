package storage

import (
	"context"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func setupTestAPIKeysDB(t *testing.T) *APIKeysDB {
	dbHost := os.Getenv("REDIS_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := 6379

	db, err := InitAPIKeysStorageDB(dbHost, dbPort)
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Очищаем базу перед тестом
	err = db.Client.FlushDB(context.Background()).Err()
	assert.NoError(t, err)

	return db
}

func TestAPIKeysDB_Set(t *testing.T) {
	db := setupTestAPIKeysDB(t)
	ctx := context.Background()
	key := "test_api_key"

	t.Run("set new api key", func(t *testing.T) {
		err := db.Set(ctx, key)
		assert.NoError(t, err)

		// Проверяем что ключ записался
		exists, err := db.exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)

		// Проверяем содержимое
		var record APIKeyRecord
		err = db.Client.HGetAll(ctx, key).Scan(&record)
		assert.NoError(t, err)
		assert.True(t, record.Valid)
	})

	t.Run("set existing api key", func(t *testing.T) {
		// Сначала создаем ключ
		err := db.Set(ctx, key)
		assert.NoError(t, err)

		// Пытаемся установить его снова
		err = db.Set(ctx, key)
		assert.NoError(t, err)

		// Проверяем что ключ все еще существует
		exists, err := db.exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestAPIKeysDB_Get(t *testing.T) {
	db := setupTestAPIKeysDB(t)
	ctx := context.Background()
	key := "test_api_key"

	t.Run("get non-existent key", func(t *testing.T) {
		record, err := db.Get(ctx, "nonexistent_key")
		assert.NoError(t, err)
		assert.False(t, record.Valid)
	})

	t.Run("get existing key", func(t *testing.T) {
		// Сначала создаем ключ
		err := db.Set(ctx, key)
		assert.NoError(t, err)

		// Получаем ключ
		record, err := db.Get(ctx, key)
		assert.NoError(t, err)
		assert.True(t, record.Valid)
	})

	t.Run("get invalid key", func(t *testing.T) {
		invalidKey := "invalid_key"
		// Создаем запись с неверной структурой
		err := db.Client.HSet(ctx, invalidKey, "valid", "not_a_boolean").Err()
		assert.NoError(t, err)

		_, err = db.Get(ctx, invalidKey)
		assert.Error(t, err) // Должна быть ошибка парсинга
	})
}

func TestAPIKeysDB_exists(t *testing.T) {
	db := setupTestAPIKeysDB(t)
	ctx := context.Background()
	key := "test_api_key"

	t.Run("check non-existent key", func(t *testing.T) {
		exists, err := db.exists(ctx, "nonexistent_key")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("check existing key", func(t *testing.T) {
		err := db.Set(ctx, key)
		assert.NoError(t, err)

		exists, err := db.exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestAPIKeysDB_Ping(t *testing.T) {
	db := setupTestAPIKeysDB(t)
	ctx := context.Background()

	t.Run("ping success", func(t *testing.T) {
		assert.True(t, db.Ping(ctx))
	})

	t.Run("ping failure", func(t *testing.T) {
		// Создаем "битый" клиент
		badDB := &APIKeysDB{
			Client: redis.NewClient(&redis.Options{
				Addr: "invalid_host:6379",
			}),
		}
		assert.False(t, badDB.Ping(ctx))
	})
}

func TestInitAPIKeysStorageDB(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		dbHost := os.Getenv("REDIS_HOST")
		if dbHost == "" {
			dbHost = "localhost"
		}
		db, err := InitAPIKeysStorageDB(dbHost, 6379)
		assert.NoError(t, err)
		assert.NotNil(t, db.Client)
	})

	t.Run("connection error", func(t *testing.T) {
		db, err := InitAPIKeysStorageDB("invalid_host", 6379)
		assert.Error(t, err)
		assert.Nil(t, db)
	})

	t.Run("connection to specific DB", func(t *testing.T) {
		dbHost := os.Getenv("REDIS_HOST")
		if dbHost == "" {
			dbHost = "localhost"
		}
		_, err := InitAPIKeysStorageDB(dbHost, 6379)
		assert.NoError(t, err)
	})
}
