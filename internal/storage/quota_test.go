//go:build integration

package storage

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.name/internal/config"
)

func TestReduceQuota(t *testing.T) {
	db := setupTestRedis(t)
	ctx := context.Background()
	logger := slog.Default()

	t.Run("create new record", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		err := db.ReduceQuota(ctx, key, logger)
		require.NoError(t, err)

		val, err := db.Client.HGet(ctx, key, "countdown").Int()
		require.NoError(t, err)
		assert.Equal(t, config.Quota-1, val)
		ttl := db.Client.TTL(ctx, key).Val()

		assert.True(t, ttl > 0)
		assert.True(t, ttl <= config.QuotaResetPeriod)
	})

	t.Run("decrement existing record", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		err := db.ReduceQuota(ctx, key, logger)
		require.NoError(t, err)

		val, err := db.Client.HGet(ctx, key, "countdown").Int()
		require.NoError(t, err)
		assert.Equal(t, config.Quota-1, val)
	})
}

func TestIsQuotaValid(t *testing.T) {
	db := setupTestRedis(t)
	ctx := context.Background()

	t.Run("no record - valid", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		valid, err := db.IsQuotaValid(ctx, key)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("record with positive countdown - valid", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		err := db.Client.HSet(ctx, key, "countdown", 1).Err()
		require.NoError(t, err)

		valid, err := db.IsQuotaValid(ctx, key)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("record with zero countdown - invalid", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		err := db.Client.HSet(ctx, key, "countdown", 0).Err()
		require.NoError(t, err)

		valid, err := db.IsQuotaValid(ctx, key)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("record with negative countdown - invalid", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		err := db.Client.HSet(ctx, key, "countdown", -1).Err()
		require.NoError(t, err)

		valid, err := db.IsQuotaValid(ctx, key)
		require.NoError(t, err)
		assert.False(t, valid)
	})
}

func TestQuotaDB_exists(t *testing.T) {
	db := setupTestRedis(t)
	ctx := context.Background()

	t.Run("key does not exist", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		exists, err := db.exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("key exists", func(t *testing.T) {
		t.Parallel()
		key := getKeyPrefix(t, "test_key")
		err := db.Client.Set(ctx, key, "value", 0).Err()
		require.NoError(t, err)

		exists, err := db.exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func setupTestRedis(t *testing.T) *QuotaDB {
	t.Helper()
	dbHost := os.Getenv("REDIS_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := 6379

	db, err := InitQuotaStorageDB(dbHost, dbPort)
	require.NoError(t, err)

	return db
}
