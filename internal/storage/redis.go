package storage

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	Client *redis.Client
}

func (db *RedisDB) Exists(ctx context.Context, key string) (bool, error) {
	keysNumber, err := db.Client.Exists(ctx, key).Uint64()
	if err != nil {
		return false, err
	}

	return keysNumber > 0, nil
}

func (db *RedisDB) Get(ctx context.Context, key string) (RecordAnswer, error) {
	var answer RecordAnswer
	exists, err := db.Exists(ctx, key)

	if err != nil {
		return answer, err
	}

	if !exists {
		return answer, ErrKeyNotFound
	}

	var record Record
	err = db.Client.HGetAll(ctx, key).Scan(&record)
	if err != nil {
		return answer, err
	}

	clicks, clicksErr := db.Client.HIncrBy(ctx, key, "clicks", 1).Result()
	if clicksErr != nil {
		return answer, clicksErr
	}
	log.Printf("Increased click counter for key '%s', now: %d", key, clicks)

	if record.Disposable {
		countdown, countdownErr := db.Client.HIncrBy(ctx, key, "countdown", -1).Result()
		if countdownErr != nil {
			panic("Fatal error when countdown: " + countdownErr.Error())
		}
		log.Printf("Decreased countdown disposable key '%s' when getting, countdown=%d", key, countdown)

		if countdown < 1 {
			delErr := db.Client.Del(ctx, key).Err()
			if delErr != nil {
				panic("Fatal error when deletion disposable url: " + delErr.Error())
			}
			log.Printf("Removed disposable key '%s' when getting", key)
		}
	}

	answer.Body = record.Body
	answer.URL = record.URL

	return answer, nil
}

func (db *RedisDB) GetClicks(ctx context.Context, key string) (int, error) {
	exists, err := db.Exists(ctx, key)
	if err != nil {
		return 0, err
	}

	if !exists {
		return 0, ErrKeyNotFound
	}

	clicks, err := db.Client.HGet(ctx, key, "clicks").Result()
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(clicks)
}

func (db *RedisDB) Set(ctx context.Context, key string, ttl time.Duration, record Record) error {
	err := db.Client.HSet(ctx, key, record).Err()
	if err != nil {
		return err
	}

	return db.Client.Expire(ctx, key, ttl).Err()
}

func (db *RedisDB) Ping(ctx context.Context) bool {
	if err := db.Client.Ping(ctx).Err(); err != nil {
		return false
	}

	return true
}

func InitStorageDB(dbHost string, dbPort int) (*RedisDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", dbHost, dbPort),
		Password:     "",
		Username:     "",
		DB:           0,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	log.Printf("Connected to database on %s:%d\n", dbHost, dbPort)

	return &RedisDB{Client: client}, nil
}
