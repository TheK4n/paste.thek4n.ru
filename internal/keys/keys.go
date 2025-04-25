package keys

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

const ATTEMPTS_TO_INCREASE_KEY_MIN_LENGHT = 20
const MAX_KEY_LENGTH = 20
const CHARSET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"


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

	uniqKey, err := generateUniqKey(ctx, db, length, MAX_KEY_LENGTH, ATTEMPTS_TO_INCREASE_KEY_MIN_LENGHT, CHARSET)
	if err != nil {
		return "", fmt.Errorf("Error on generating unique key: %w", err)
	}

	err = db.Set(ctx, uniqKey, ttl, record)
	if err != nil {
		return "", fmt.Errorf("Error on setting key '%s' in db: %w", uniqKey, err)
	}

	return uniqKey, nil
}

// Generates unique key with minimum lenght of minLength using charset
// increases minLength if was attemptsToIncreaseMinLength attempts generate unique key
// Return error if database error or context done or maxLength reached
func generateUniqKey(
	ctx context.Context, db storage.RedisDB,
	minLength int, maxLength int,
	attemptsToIncreaseMinLength int,
	charset string,
) (string, error) {
	key := generateKey(minLength, charset)
	exists, err := db.Exists(ctx, key)
	if err != nil {
		return "", err
	}
	currentAttemptsCountdown := attemptsToIncreaseMinLength

	for exists {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("Timeout")
		default:
		}

		key = generateKey(minLength, charset)
		exists, err = db.Exists(ctx, key)
		if err != nil {
			return "", err
		}
		currentAttemptsCountdown--

		if currentAttemptsCountdown < 1 {
			minLength++
			currentAttemptsCountdown = attemptsToIncreaseMinLength
		}

		if minLength > maxLength {
			return "", fmt.Errorf("Max key length reached")
		}
	}

	return key, nil
}

func generateKey(length int, charset string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	charsetLen := len(charset)

	for i := range length {
		result[i] = charset[r.Intn(charsetLen)]
	}

	return string(result)
}
