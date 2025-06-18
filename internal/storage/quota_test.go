//go:build integration

package storage

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thek4n/paste.thek4n.name/internal/config"
)

func setupTestRedis(t *testing.T) *QuotaDB {
	dbHost := os.Getenv("REDIS_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := 6379

	db, err := InitQuotaStorageDB(dbHost, dbPort)
	if err != nil {
		panic(err)
	}

	err = db.Client.FlushDB(context.Background()).Err()
	assert.NoError(t, err)

	return db
}

func TestQuotaDB_CreateAndSubOrJustSub(t *testing.T) {
	db := setupTestRedis(t)
	ctx := context.Background()
	key := "test_key"

	t.Run("create new record", func(t *testing.T) {
		err := db.CreateAndSubOrJustSub(ctx, key)
		assert.NoError(t, err)

		val, err := db.Client.HGet(ctx, key, "countdown").Int()
		assert.NoError(t, err)
		assert.Equal(t, config.QUOTA-1, val)
		ttl := db.Client.TTL(ctx, key).Val()

		assert.True(t, ttl > 0)
		assert.True(t, ttl <= config.QUOTA_PERIOD)
	})

	t.Run("decrement existing record", func(t *testing.T) {
		err := db.CreateAndSubOrJustSub(ctx, key)
		assert.NoError(t, err)

		val, err := db.Client.HGet(ctx, key, "countdown").Int()
		assert.NoError(t, err)
		assert.Equal(t, config.QUOTA-2, val)
	})
}

func TestQuotaDB_IsQuotaValid(t *testing.T) {
	db := setupTestRedis(t)
	ctx := context.Background()
	key := "test_key"

	t.Run("no record - valid", func(t *testing.T) {
		valid, err := db.IsQuotaValid(ctx, key)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("record with positive countdown - valid", func(t *testing.T) {
		err := db.Client.HSet(ctx, key, "countdown", 1).Err()
		assert.NoError(t, err)

		valid, err := db.IsQuotaValid(ctx, key)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("record with zero countdown - invalid", func(t *testing.T) {
		err := db.Client.HSet(ctx, key, "countdown", 0).Err()
		assert.NoError(t, err)

		valid, err := db.IsQuotaValid(ctx, key)
		assert.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("record with negative countdown - invalid", func(t *testing.T) {
		err := db.Client.HSet(ctx, key, "countdown", -1).Err()
		assert.NoError(t, err)

		valid, err := db.IsQuotaValid(ctx, key)
		assert.NoError(t, err)
		assert.False(t, valid)
	})
}

func TestQuotaDB_exists(t *testing.T) {
	db := setupTestRedis(t)
	ctx := context.Background()
	key := "test_key"

	t.Run("key does not exist", func(t *testing.T) {
		exists, err := db.exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("key exists", func(t *testing.T) {
		err := db.Client.Set(ctx, key, "value", 0).Err()
		assert.NoError(t, err)

		exists, err := db.exists(ctx, key)
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}
