// Package repository contains implementaions of domain repository interfaces.
package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

type redisQuotaRecord struct {
	Value int32 `redis:"value"`
}

// RedisQuotaRepository implementation of domain interface of quota repository.
type RedisQuotaRepository struct {
	client *redis.Client
	config config.QuotaConfig
}

// NewRedisQuotaRepository constructor.
func NewRedisQuotaRepository(c *redis.Client, cfg config.QuotaConfig) *RedisQuotaRepository {
	return &RedisQuotaRepository{
		client: c,
		config: cfg,
	}
}

// GetByID get Quota aggregate from db.
func (r *RedisQuotaRepository) GetByID(ctx context.Context, id objectvalue.QuotaSourceIP) (aggregate.Quota, error) {
	var quota redisQuotaRecord

	exists, err := r.exists(ctx, string(id))
	if err != nil {
		return aggregate.Quota{}, err
	}

	if !exists {
		return aggregate.Quota{}, domainerrors.ErrQuotaNotFound
	}

	err = r.client.HGetAll(ctx, string(id)).Scan(&quota)
	if err != nil {
		return aggregate.Quota{}, fmt.Errorf("failure get quota for ip '%s': %w", id, err)
	}

	q, err := aggregate.NewQuota(id, quota.Value)
	if err != nil {
		return aggregate.Quota{}, fmt.Errorf("failure construct quota: %w", err)
	}

	return q, nil
}

// SetByID write quota to db.
func (r *RedisQuotaRepository) SetByID(ctx context.Context, id objectvalue.QuotaSourceIP, q aggregate.Quota) error {
	// lua script because we need atomic execution
	script := `
	   redis.call("HSET", KEYS[1], ARGV[1], ARGV[2])
	   redis.call("EXPIRE", KEYS[1], ARGV[3])
	   return 1
   `
	err := r.client.Eval(ctx, script, []string{string(id)}, "value", q.Value(), int(r.config.QuotaResetPeriod().Seconds())).Err()
	if err != nil {
		return fmt.Errorf("fail to set new quota key: %w", err)
	}

	return nil
}

func (r *RedisQuotaRepository) exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := r.client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, fmt.Errorf("failure checking is quota exists: %w", err)
	}

	return keysNumber > 0, nil
}
