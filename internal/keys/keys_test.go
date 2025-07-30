//go:build integration

package keys

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.ru/internal/storage"
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

func setup() {
	dbHost := getRedisHost()
	dbPort := 6379
	db, _ := storage.InitKeysStorageDB(dbHost, dbPort)
	_ = db.Client.FlushDB(context.Background()).Err()
}

func TestCache(t *testing.T) {
	db := setupTestKeysDB(t)
	timeout := 1 * time.Second
	ttl := 0 * time.Second
	record := storage.KeyRecord{Body: []byte("test")}

	t.Run("requested key and got key are equal", func(t *testing.T) {
		t.Parallel()

		expectedKey := getKeyPrefix(t, "test_key")
		gotKey, err := CacheRequestedKey(db, timeout, expectedKey, ttl, record)

		require.NoError(t, err)
		assert.Equal(t, expectedKey, gotKey)
	})

	t.Run("attempt to take already taken key returns error", func(t *testing.T) {
		t.Parallel()

		expectedKey := getKeyPrefix(t, "taken_key")
		_, errCaching := CacheRequestedKey(db, timeout, expectedKey, ttl, record)

		_, errCachingSecond := CacheRequestedKey(db, timeout, expectedKey, ttl, record)

		require.NoError(t, errCaching)
		assert.Error(t, errCachingSecond)
		assert.Equal(t, ErrKeyAlreadyTaken, errCachingSecond)
	})

	t.Run("CacheGeneratedKey returns generated key with requested length", func(t *testing.T) {
		t.Parallel()

		keyLength := 10
		gotKey, errCaching := CacheGeneratedKey(db, timeout, ttl, keyLength, record)
		require.NoError(t, errCaching)

		assert.Len(t, gotKey, keyLength)
	})

	t.Run("get by generated random key has expected body", func(t *testing.T) {
		t.Parallel()

		keyLength := 10
		gotKey, errCaching := CacheGeneratedKey(db, timeout, ttl, keyLength, record)
		require.NoError(t, errCaching)

		result, err := Get(db, gotKey, timeout)
		require.NoError(t, err)
		assert.Equal(t, record.Body, result.Body)
	})
}

func TestGet(t *testing.T) {
	db := setupTestKeysDB(t)
	record := storage.KeyRecord{Body: []byte("test")}
	timeout := 1 * time.Second
	ttl := 0 * time.Second

	t.Run("requested key and got key are equal", func(t *testing.T) {
		t.Parallel()

		key := "nonexistent_key"
		timeout := 1 * time.Second

		_, err := Get(db, key, timeout)

		require.Error(t, err)
		assert.Equal(t, storage.ErrKeyNotFound, errors.Unwrap(err))
	})

	t.Run("record clicks equal number of get requests", func(t *testing.T) {
		t.Parallel()

		expectedKey := getKeyPrefix(t, "taken_key")
		requestNumber := 3
		_, errCaching := CacheRequestedKey(db, timeout, expectedKey, ttl, record)
		require.NoError(t, errCaching)

		for range requestNumber {
			_, err := Get(db, expectedKey, timeout)
			require.NoError(t, err)
		}

		clicks, err := GetClicks(db, expectedKey, timeout)
		require.NoError(t, err)
		assert.Equal(t, requestNumber, clicks)
	})

	t.Run("get by non existent key returns error", func(t *testing.T) {
		t.Parallel()

		key := "nonexistent_key"

		_, err := GetClicks(db, key, timeout)

		assert.Error(t, err)
		assert.Equal(t, storage.ErrKeyNotFound, errors.Unwrap(err))
	})
}

func setupTestKeysDB(t *testing.T) storage.KeysDB {
	t.Helper()
	dbHost := getRedisHost()
	dbPort := 6379

	db, err := storage.InitKeysStorageDB(dbHost, dbPort)
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
