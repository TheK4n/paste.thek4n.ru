//go:build integration

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

	// Clean db before test
	err = db.Client.FlushDB(context.Background()).Err()
	if err != nil {
		t.Fatalf("Failed to flush db: %v", err)
	}

	return db
}

func TestRequestedKeyAndGotKeyAreEqual(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "test_key"
	body := []byte("test body")
	timeout := 1 * time.Second

	gotKey, err := Cache(db, timeout, key, 0*time.Second, len(key), storage.KeyRecord{Body: body})

	assert.NoError(t, err)
	assert.Equal(t, key, gotKey)
}

func TestCachedAndGottenBodyAreEqual(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "test_key"
	body := []byte("test body")
	timeout := 1 * time.Second

	_, errCaching := Cache(db, timeout, key, 0*time.Second, len(key), storage.KeyRecord{Body: body})
	result, errGetting := Get(db, key, timeout)

	assert.NoError(t, errCaching)
	assert.NoError(t, errGetting)
	assert.Equal(t, body, result.Body)
}

func TestNotExistingKeyIsNotFound(t *testing.T) {
	db := *setupTestKeysDB(t)
	key := "nonexistent_key"
	timeout := 1 * time.Second

	_, err := Get(db, key, timeout)

	assert.Error(t, err)
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

	assert.NoError(t, errCaching)
	assert.NoError(t, errGetting)
	clicks, err := GetClicks(db, key, timeout)
	assert.NoError(t, err)
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

	assert.NoError(t, errCaching)
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

	assert.NoError(t, errCaching)
	assert.NotEmpty(t, gotKey)
	assert.Len(t, gotKey, keyLength)

	result, err := Get(db, gotKey, timeout)
	assert.NoError(t, err)
	assert.Equal(t, record.Body, result.Body)
}
