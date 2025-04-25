package keys

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

func Get(db storage.RedisDB, key string, timeout time.Duration) (storage.RecordAnswer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return db.Get(ctx, key)
}

func GetClicks(db storage.RedisDB, key string, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return db.GetClicks(ctx, key)
}

func Cache(db storage.RedisDB, timeout time.Duration, ttl time.Duration, length int, record storage.Record) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	uniqKey, err := generateUniqKey(ctx, db, length)
	if err != nil {
		return "", fmt.Errorf("Error on generating unique key: %w", err)
	}

	err = db.Set(ctx, uniqKey, ttl, record)
	if err != nil {
		return "", fmt.Errorf("Error on setting key '%s' in db: %w", uniqKey, err)
	}

	return uniqKey, nil
}

func generateUniqKey(ctx context.Context, db storage.RedisDB, length int) (string, error) {
	key := generateKey(length)
	exists, err := db.Exists(ctx, key)
	if err != nil {
		return "", err
	}

	for exists {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("Timeout")
		default:
		}

		key = generateKey(length)
		exists, err = db.Exists(ctx, key)
		if err != nil {
			return "", err
		}
	}

	return key, nil
}

func generateKey(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range length {
		result[i] = chars[r.Intn(len(chars))]
	}

	return string(result)
}
