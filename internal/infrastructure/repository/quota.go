// Package repository contains implementaions of domain repository interfaces.
package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

type redisQuotaRecord struct {
	Value int32 `redis:"value"`
}

// RedisQuotaRepository implementation of domain interface of quota repository.
type RedisQuotaRepository struct {
	client *redis.Client
}

// NewRedisQuotaRepository constructor.
func NewRedisQuotaRepository(c *redis.Client) *RedisQuotaRepository {
	return &RedisQuotaRepository{
		client: c,
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
	record := redisQuotaRecord{
		Value: q.Value(),
	}

	err := r.client.HSet(ctx, string(id), record).Err()
	if err != nil {
		return fmt.Errorf("fail to write quota record: %w", err)
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
