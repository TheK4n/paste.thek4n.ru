package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thek4n/paste.thek4n.name/internal/config"
)

type QuotaRecord struct {
	Countdown int `redis:"countdown"`
}

type QuotaDB struct {
	Client *redis.Client
}

func (db *QuotaDB) CreateAndSubOrJustSub(ctx context.Context, key string) error {
	exists, err := db.exists(ctx, key)
	if err != nil {
		return err
	}

	if !exists {
		record := QuotaRecord{
			Countdown: config.QUOTA,
		}

		// TODO: обернуть 2 нижележащие инструкции в транзакцию (мы не хотим, чтобы случайно забанился навечно айпишник)
		err := db.Client.HSet(ctx, key, record).Err()
		if err != nil {
			return err
		}

		err = db.Client.Expire(ctx, key, config.QUOTA_PERIOD).Err()
		if err != nil {
			return err
		}
	}

	return db.Client.HIncrBy(ctx, key, "countdown", -1).Err()
}

func (db *QuotaDB) IsQuotaValid(ctx context.Context, key string) (bool, error) {
	exists, err := db.exists(ctx, key)
	if err != nil {
		return false, err
	}
	if !exists {
		return true, nil
	}

	res, err := db.Client.HGet(ctx, key, "countdown").Int()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

func (db *QuotaDB) exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := db.Client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, err
	}

	return keysNumber > 0, nil
}

func InitQuotaStorageDB(dbHost string, dbPort int) (*QuotaDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", dbHost, dbPort),
		Password:     "",
		Username:     "",
		DB:           2,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	log.Printf("Connected to database 2 (quota) on %s:%d\n", dbHost, dbPort)

	return &QuotaDB{Client: client}, nil
}
