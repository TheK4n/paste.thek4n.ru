//go:build integration

package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.ru/internal/config"
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func setup() {
	dbHost := getRedisHost()
	dbPort := 6379
	db, _ := InitKeysStorageDB(dbHost, dbPort)
	_ = db.Client.FlushDB(context.Background()).Err()

	dbq, _ := InitQuotaStorageDB(dbHost, dbPort)
	_ = dbq.Client.FlushDB(context.Background()).Err()
}

func TestKeysDB_Exists(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()

	t.Run("key does not exist", func(t *testing.T) {
		t.Parallel()
		key := "nonexistent_key"

		exists, err := db.Exists(ctx, key)

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("key exists", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		err := db.Client.Set(ctx, key, "value", 0).Err()
		require.NoError(t, err)

		exists, err := db.Exists(ctx, key)

		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestKeysDB_Get(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()
	testBody := []byte("test body")

	t.Run("key not found", func(t *testing.T) {
		t.Parallel()
		_, err := db.Get(ctx, "nonexistent_key")
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("get non-disposable key", func(t *testing.T) {
		record := KeyRecord{
			URL:    true,
			Body:   testBody,
			Clicks: 0,
		}
		key := getKeyPrefix(t, "test_key")
		err := db.Set(ctx, key, 0, record)
		require.NoError(t, err)

		result, err := db.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, testBody, result.Body)
		assert.True(t, result.URL)

		clicks, err := db.GetClicks(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 1, clicks)
	})

	t.Run("get disposable key", func(t *testing.T) {
		disposableKey := getKeyPrefix(t, "disposable_key")
		record := KeyRecord{
			Disposable: true,
			Countdown:  1,
			Body:       testBody,
		}
		err := db.Set(ctx, disposableKey, 0, record)
		require.NoError(t, err)

		result, err := db.Get(ctx, disposableKey)
		require.NoError(t, err)
		assert.Equal(t, testBody, result.Body)

		exists, err := db.Exists(ctx, disposableKey)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("get compressed data", func(t *testing.T) {
		compressedKey := getKeyPrefix(t, "compressed_key")
		largeBody := bytes.Repeat([]byte("a"), config.CompressThresholdBytes+1)
		record := KeyRecord{
			Body: largeBody,
		}

		err := db.Set(ctx, compressedKey, 0, record)
		require.NoError(t, err)

		result, err := db.Get(ctx, compressedKey)
		require.NoError(t, err)
		assert.Equal(t, largeBody, result.Body)
	})
}

func TestKeysDB_GetClicks(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()
	expectedClicks := 5

	t.Run("key not found", func(t *testing.T) {
		_, err := db.GetClicks(ctx, "nonexistent_key")
		assert.Equal(t, ErrKeyNotFound, err)
	})

	t.Run("get clicks count", func(t *testing.T) {
		key := getKeyPrefix(t, "test_key")
		record := KeyRecord{
			Body:   []byte("test"),
			Clicks: expectedClicks,
		}
		err := db.Set(ctx, key, 0, record)
		require.NoError(t, err)

		gotClicks, err := db.GetClicks(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, expectedClicks, gotClicks)
	})
}

func TestKeysDB_Set(t *testing.T) {
	db := setupTestKeysDB(t)
	ctx := context.Background()
	testBody := []byte("test body")

	t.Run("set simple record", func(t *testing.T) {
		key := getKeyPrefix(t, "test_key")
		record := KeyRecord{
			URL:  true,
			Body: testBody,
		}
		err := db.Set(ctx, key, 0, record)
		require.NoError(t, err)

		var result KeyRecord
		err = db.Client.HGetAll(ctx, key).Scan(&result)
		require.NoError(t, err)
		assert.Equal(t, testBody, result.Body)
		assert.True(t, result.URL)
	})

	t.Run("set with TTL", func(t *testing.T) {
		ttl := 10 * time.Second
		key := getKeyPrefix(t, "ttl_key")
		record := KeyRecord{
			Body: testBody,
		}
		err := db.Set(ctx, key, ttl, record)
		require.NoError(t, err)

		actualTTL := db.Client.TTL(ctx, key).Val()
		assert.True(t, actualTTL > 0)
		assert.True(t, actualTTL <= ttl)
	})

	t.Run("auto compress large data", func(t *testing.T) {
		key := getKeyPrefix(t, "large_key")
		largeBody := bytes.Repeat([]byte("a"), config.CompressThresholdBytes+1)
		record := KeyRecord{
			Body: largeBody,
		}
		err := db.Set(ctx, key, 0, record)
		require.NoError(t, err)

		var result KeyRecord
		err = db.Client.HGetAll(ctx, key).Scan(&result)
		require.NoError(t, err)
		assert.True(t, isCompressed(result.Body))
	})
}

func setupTestKeysDB(t *testing.T) KeysDB {
	t.Helper()
	dbHost := getRedisHost()
	dbPort := 6379

	db, err := InitKeysStorageDB(dbHost, dbPort)
	require.NoError(t, err, "Failed to setup keys db")

	return *db
}

func getRedisHost() string {
	dbHost := os.Getenv("REDIS_HOST")
	if dbHost == "" {
		return "localhost"
	}
	return dbHost
}

func getKeyPrefix(t *testing.T, key string) string {
	return fmt.Sprintf("%s:%s", t.Name(), key)
}
