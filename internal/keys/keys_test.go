//go:build integration

package keys

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

func setupTestKeysDB(t *testing.T) *storage.KeysDB {
	t.Helper()
	dbHost := os.Getenv("REDIS_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := 6379

	db, err := storage.InitKeysStorageDB(dbHost, dbPort)
	require.NoError(t, err, "Failed to setup keys db")

	err = db.Client.FlushDB(context.Background()).Err()
	require.NoError(t, err, "Failed to flush db")

	return db
}

func TestRequestedKeyAndGotKeyAreEqual(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "test_key"
	body := []byte("test body")
	timeout := 1 * time.Second

	gotKey, err := Cache(db, timeout, key, 0*time.Second, len(key), storage.KeyRecord{Body: body})

	require.NoError(t, err)
	assert.Equal(t, key, gotKey)
}

func TestCachedAndGottenBodyAreEqual(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "test_key"
	body := []byte("test body")
	timeout := 1 * time.Second

	_, errCaching := Cache(db, timeout, key, 0*time.Second, len(key), storage.KeyRecord{Body: body})
	result, errGetting := Get(db, key, timeout)

	require.NoError(t, errCaching)
	require.NoError(t, errGetting)
	assert.Equal(t, body, result.Body)
}

func TestNotExistingKeyIsNotFound(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "nonexistent_key"
	timeout := 1 * time.Second

	_, err := Get(db, key, timeout)

	require.Error(t, err)
	assert.Equal(t, storage.ErrKeyNotFound, err)
}

func TestGetClicksEqualNumberOfGetRequests(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "test_key"
	body := []byte("test body")
	requestNumber := 3
	timeout := 1 * time.Second
	_, errCaching := Cache(db, timeout, key, 0*time.Second, len(key), storage.KeyRecord{Body: body})
	var errGetting error

	for range requestNumber {
		_, errGetting = Get(db, key, timeout)
	}

	require.NoError(t, errCaching)
	require.NoError(t, errGetting)
	clicks, err := GetClicks(db, key, timeout)
	require.NoError(t, err)
	assert.Equal(t, requestNumber, clicks)
}

func TestGetClicksForNotExistingKeyIsNotFound(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "nonexistent_key"
	timeout := 1 * time.Second

	_, err := GetClicks(db, key, timeout)

	assert.Error(t, err)
	assert.Equal(t, storage.ErrKeyNotFound, err)
}

func TestRequestedKeyAlreadyTaken(t *testing.T) {
	db := *setupTestKeysDB(t)
	timeout := 1 * time.Second
	requestedKey := "taken_key"
	record := storage.KeyRecord{Body: []byte("test")}
	_, errCaching := Cache(db, timeout, requestedKey, 0*time.Second, 10, record)

	_, errCachingSecond := Cache(db, timeout, requestedKey, 0*time.Second, 10, record)

	require.NoError(t, errCaching)
	assert.Error(t, errCachingSecond)
	assert.Equal(t, ErrKeyAlreadyTaken, errCachingSecond)
}

func TestSuccessfulCacheWithGeneratedKey(t *testing.T) {
	db := *setupTestKeysDB(t)
	timeout := 1 * time.Second
	requestedKey := ""
	keyLength := 10
	record := storage.KeyRecord{Body: []byte("test")}

	gotKey, errCaching := Cache(db, timeout, requestedKey, 0*time.Second, keyLength, record)

	require.NoError(t, errCaching)
	assert.NotEmpty(t, gotKey)
	assert.Len(t, gotKey, keyLength)

	result, err := Get(db, gotKey, timeout)
	require.NoError(t, err)
	assert.Equal(t, record.Body, result.Body)
}
