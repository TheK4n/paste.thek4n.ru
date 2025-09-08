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
	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/repository"
)

func TestCacheService_Serve(t *testing.T) {
	t.Parallel()

	publisher := event.NewPublisher()

	recordRepo := repository.NewRedisRecordRepository(newRedisClient(0), config.DefaultCachingConfig{})
	quotaRepo := repository.NewRedisQuotaRepository(newRedisClient(1), config.DefaultQuotaConfig{})
	apikeyRepo := repository.NewRedisAPIKeyRORepository(newRedisClient(2))

	apikeyService := TrueAPIKeyService{}

	cacheValidationCfg := config.DefaultCacheValidationConfig{}

	// Setup service
	svc := NewCacheService(
		recordRepo,
		quotaRepo,
		apikeyRepo,
		apikeyService,
		publisher,
		cacheValidationCfg,
		config.DefaultQuotaConfig{},
		MuteLogger{},
	)

	t.Run("successful generation with correct values", func(t *testing.T) {
		params := objectvalue.CacheRequestParams{
			APIKey:             "",
			RequestedKey:       "",
			SourceIP:           "127.0.0.1",
			Body:               []byte("test"),
			TTL:                cacheValidationCfg.DefaultTTL(),
			BodyLen:            4,
			RequestedKeyLength: cacheValidationCfg.DefaultKeyLength(),
			Disposable:         1,
			IsURL:              false,
		}
		key, err := svc.Serve(params)
		require.NoError(t, err)

		assert.NotEmpty(t, key)
	})

	t.Run("service returns correct requested key with apikey mock", func(t *testing.T) {
		params := objectvalue.CacheRequestParams{
			APIKey:             "non-empty",
			RequestedKey:       "key",
			SourceIP:           "127.0.0.1",
			Body:               []byte("test"),
			TTL:                cacheValidationCfg.DefaultTTL(),
			BodyLen:            4,
			RequestedKeyLength: cacheValidationCfg.DefaultKeyLength(),
			Disposable:         1,
			IsURL:              false,
		}
		key, err := svc.Serve(params)
		require.NoError(t, err)

		assert.Equal(t, "key", string(key))
	})
}

func newRedisClient(db int) *redis.Client {
	host := getRedisHost()
	port := 6379
	return redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		PoolSize:     100,
		Password:     "",
		Username:     "",
		DB:           db,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
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

type TrueAPIKeyService struct{}

func (s TrueAPIKeyService) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (s TrueAPIKeyService) CheckValid(context.Context, string) (bool, error) {
	return true, nil
}

func (s TrueAPIKeyService) GetID(context.Context, string) (string, error) {
	return "", nil
}

type FalseAPIKeyService struct{}

func (s FalseAPIKeyService) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (s FalseAPIKeyService) CheckValid(context.Context, string) (bool, error) {
	return false, nil
}

func (s FalseAPIKeyService) GetID(context.Context, string) (string, error) {
	return "", nil
}
