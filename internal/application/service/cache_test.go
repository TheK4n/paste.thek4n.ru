//go:build integration

package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/repository"
)

func TestCacheService_ServePrivileged(t *testing.T) {
	t.Parallel()

	publisher := event.NewPublisher()

	recordRedisClient := newRedisClient(t, 10)
	quotaRedisClient := newRedisClient(t, 11)
	apiKeyRedisClient := newRedisClient(t, 12)

	recordRepo := repository.NewRedisRecordRepository(recordRedisClient, config.DefaultCachingConfig{})
	quotaRepo := repository.NewRedisQuotaRepository(quotaRedisClient, config.DefaultQuotaConfig{})
	apikeyRepo := repository.NewRedisAPIKeyRORepository(apiKeyRedisClient)

	cacheValidationCfg := config.DefaultCacheValidationConfig{}

	svcAPIKeyDenier := NewCacheService(
		recordRepo,
		quotaRepo,
		apikeyRepo,
		DenierAPIKeyServiceMock{},
		publisher,
		cacheValidationCfg,
		config.DefaultQuotaConfig{},
		MuteLogger{},
	)

	svcAPIKeyPasser := NewCacheService(
		recordRepo,
		quotaRepo,
		apikeyRepo,
		PasserAPIKeyServiceMock{},
		publisher,
		cacheValidationCfg,
		config.DefaultQuotaConfig{},
		MuteLogger{},
	)

	testBody := []byte("test")

	t.Run("successful generation with correct values", func(t *testing.T) {
		params := objectvalue.CacheRequestParams{
			APIKey:             "",
			RequestedKey:       "",
			SourceIP:           "127.0.0.1",
			Body:               testBody,
			TTL:                cacheValidationCfg.DefaultTTL(),
			BodyLen:            int64(len(testBody)),
			RequestedKeyLength: cacheValidationCfg.DefaultKeyLength(),
			Disposable:         0,
			IsURL:              false,
		}
		key, err := svcAPIKeyDenier.Serve(params)
		require.NoError(t, err)

		assert.NotEmpty(t, key)
	})

	t.Run("service returns correct requested key with apikey mock-passer", func(t *testing.T) {
		params := objectvalue.CacheRequestParams{
			APIKey:             "non-empty",
			RequestedKey:       "key",
			SourceIP:           "127.0.0.1",
			Body:               testBody,
			TTL:                cacheValidationCfg.DefaultTTL(),
			BodyLen:            int64(len(testBody)),
			RequestedKeyLength: cacheValidationCfg.DefaultKeyLength(),
			Disposable:         0,
			IsURL:              false,
		}
		key, err := svcAPIKeyPasser.Serve(params)
		require.NoError(t, err)

		assert.Equal(t, "key", string(key))
	})

	t.Run("service returns non authorized with apikey denier mock and requested custom key", func(t *testing.T) {
		params := objectvalue.CacheRequestParams{
			APIKey:             "",
			RequestedKey:       "key",
			SourceIP:           "127.0.0.1",
			Body:               testBody,
			TTL:                cacheValidationCfg.DefaultTTL(),
			BodyLen:            int64(len(testBody)),
			RequestedKeyLength: cacheValidationCfg.DefaultKeyLength(),
			Disposable:         0,
			IsURL:              false,
		}
		_, err := svcAPIKeyDenier.Serve(params)
		assert.ErrorIs(t, err, domainerrors.ErrNonAuthorized)
	})

	t.Run("service successfully returns short key with passer apikey mock", func(t *testing.T) {
		keyLength := 4

		params := objectvalue.CacheRequestParams{
			APIKey:             "non-empty",
			RequestedKey:       "",
			SourceIP:           "127.0.0.1",
			Body:               testBody,
			TTL:                0,
			BodyLen:            int64(len(testBody)),
			RequestedKeyLength: uint8(keyLength),
			Disposable:         0,
			IsURL:              false,
		}
		key, err := svcAPIKeyPasser.Serve(params)
		require.NoError(t, err)

		assert.Len(t, key, keyLength)
	})
}

func newRedisClient(t *testing.T, db int) *redis.Client {
	host := getRedisHost()
	port := 6379
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		PoolSize:     100,
		Password:     "",
		Username:     "",
		DB:           db,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	require.NoError(t, client.FlushDB(context.Background()).Err())

	return client
}

func getRedisHost() string {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		return "localhost"
	}
	return redisHost
}

type MuteLogger struct{}

func (l MuteLogger) Debug(string, ...any) {}
func (l MuteLogger) Error(string, ...any) {}
func (l MuteLogger) Info(string, ...any)  {}
func (l MuteLogger) Warn(string, ...any)  {}

type PasserAPIKeyServiceMock struct{}

func (s PasserAPIKeyServiceMock) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (s PasserAPIKeyServiceMock) CheckValid(context.Context, string) (bool, error) {
	return true, nil
}

func (s PasserAPIKeyServiceMock) GetID(context.Context, string) (string, error) {
	return "", nil
}

type DenierAPIKeyServiceMock struct{}

func (s DenierAPIKeyServiceMock) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (s DenierAPIKeyServiceMock) CheckValid(context.Context, string) (bool, error) {
	return false, nil
}

func (s DenierAPIKeyServiceMock) GetID(context.Context, string) (string, error) {
	return "", nil
}
