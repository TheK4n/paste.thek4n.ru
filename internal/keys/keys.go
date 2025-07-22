// Package keys is package for requesting keys from db
package keys

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

// ErrKeyAlreadyTaken is error when key is already taken.
var ErrKeyAlreadyTaken = errors.New("key already taken")

// Get key from db using timeout for context.
func Get(db storage.KeysDB, key string, timeout time.Duration) (storage.KeyRecordAnswer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	answ, err := db.Get(ctx, key)
	if err != nil {
		return answ, fmt.Errorf("failure getting key '%s': %w", key, err)
	}

	return answ, nil
}

// GetClicks for key from db using timeout for context.
func GetClicks(db storage.KeysDB, key string, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	anws, err := db.GetClicks(ctx, key)
	if err != nil {
		return anws, fmt.Errorf("failure getting key '%s' clicks: %w", key, err)
	}

	return anws, nil
}

// CacheRequestedKey record using timeout for context.
// requestedKey - you can request custom key, if it exists func returns error ErrKeyAlreadyTaken.
// ttl - time to live for key, after this time, key will automatically deletes.
func CacheRequestedKey(
	db storage.KeysDB,
	timeout time.Duration,
	requestedKey string,
	ttl time.Duration,
	record storage.KeyRecord,
) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	exists, err := db.Exists(context.Background(), requestedKey)
	if err != nil {
		return "", fmt.Errorf("error on checking key: %w", err)
	}

	if exists {
		return "", ErrKeyAlreadyTaken
	}

	err = db.Set(ctx, requestedKey, ttl, record)
	if err != nil {
		return "", fmt.Errorf("error on setting key '%s' in db: %w", requestedKey, err)
	}

	return requestedKey, nil
}

// CacheGeneratedKey record using timeout for context.
// ttl - time to live for key, after this time, key will automatically deletes.
func CacheGeneratedKey(
	db storage.KeysDB,
	timeout time.Duration,
	ttl time.Duration,
	length int,
	record storage.KeyRecord,
) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	uniqKey, err := generateUniqKey(
		ctx,
		db,
		length,
		config.MaxKeyLength,
		config.AttemptsToIncreaseKeyMinLenght,
		config.Charset,
	)
	if err != nil {
		return "", fmt.Errorf("error on generating unique key: %w", err)
	}

	err = db.Set(ctx, uniqKey, ttl, record)
	if err != nil {
		return "", fmt.Errorf("error on setting key '%s' in db: %w", uniqKey, err)
	}

	return uniqKey, nil
}

// Generates unique key with minimum length of minLength using charset.
// increases minLength if was attemptsToIncreaseMinLength attempts generate unique key.
// Return error if database error or context done or maxLength reached.
func generateUniqKey(
	ctx context.Context, db storage.KeysDB,
	minLength int, maxLength int,
	attemptsToIncreaseMinLength int,
	charset string,
) (string, error) {
	var err error
	var key string
	currentAttemptsCountdown := attemptsToIncreaseMinLength

	// initial true for start cycle
	exists := true
	for exists {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout")
		default:
		}

		key, err = generateKey(minLength, charset)
		if err != nil {
			return "", fmt.Errorf("fail generate key: %w", err)
		}
		exists, err = db.Exists(ctx, key)
		if err != nil {
			return "", fmt.Errorf("fail to check is key '%s' exists: %w", key, err)
		}
		currentAttemptsCountdown--

		if currentAttemptsCountdown < 1 {
			minLength++
			currentAttemptsCountdown = attemptsToIncreaseMinLength
		}

		if minLength > maxLength {
			return "", fmt.Errorf("max key length reached")
		}
	}

	return key, nil
}

// generateKey generate new random key with specified length using charset.
func generateKey(length int, charset string) (string, error) {
	result := make([]byte, length)
	charsetLen := len(charset)

	for i := range length {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(charsetLen)))
		if err != nil {
			return "", fmt.Errorf("failure generate random number: %w", err)
		}
		result[i] = charset[nBig.Int64()]
	}

	return string(result), nil
}
