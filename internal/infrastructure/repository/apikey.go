package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

type redisAPIKeyRecord struct {
	ID    string `redis:"id"`
	Valid bool   `redis:"valid"`
}

// RedisAPIKeyRORepository redis implementation domain interface.
type RedisAPIKeyRORepository struct {
	client *redis.Client
}

// NewRedisAPIKeyRORepository constructor.
func NewRedisAPIKeyRORepository(c *redis.Client) *RedisAPIKeyRORepository {
	return &RedisAPIKeyRORepository{
		client: c,
	}
}

// GetByID fetch APIKey from redis db.
func (r *RedisAPIKeyRORepository) GetByID(ctx context.Context, key string) (aggregate.APIKey, error) {
	var record redisAPIKeyRecord

	exists, err := r.exists(ctx, key)
	if err != nil {
		return aggregate.APIKey{}, err
	}

	if !exists {
		return aggregate.APIKey{}, domainerrors.ErrAPIKeyNotFound
	}

	err = r.client.HGetAll(ctx, key).Scan(&record)
	if err != nil {
		return aggregate.APIKey{}, fmt.Errorf("failure get record for key '%s': %w", key, err)
	}

	rid, err := objectvalue.NewAPIKeyID(record.ID)
	if err != nil {
		return aggregate.APIKey{}, fmt.Errorf("fail to parse apikey record id for key '%s': %w", key, err)
	}

	return aggregate.NewAPIKey(rid, key, record.Valid), nil
}

func (r *RedisAPIKeyRORepository) exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := r.client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, fmt.Errorf("failure checking is key exists: %w", err)
	}

	return keysNumber > 0, nil
}

// RedisAPIKeyWORepository redis implementation of domain interface.
type RedisAPIKeyWORepository struct {
	client *redis.Client
}

// NewRedisAPIKeyWORepository constructor.
func NewRedisAPIKeyWORepository(c *redis.Client) *RedisAPIKeyWORepository {
	return &RedisAPIKeyWORepository{
		client: c,
	}
}

// SetByID write apikey to redis.
func (r *RedisAPIKeyWORepository) SetByID(ctx context.Context, key string, apikey aggregate.APIKey) error {
	record := redisAPIKeyRecord{
		ID:    apikey.PublicID().String(),
		Valid: apikey.Valid(),
	}

	err := r.client.HSet(ctx, key, record).Err()
	if err != nil {
		return fmt.Errorf("failure set record for key '%s': %w", key, err)
	}

	return nil
}
