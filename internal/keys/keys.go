package keys

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

func Get(db storage.KeysDB, key string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return db.Get(ctx, key)
}

func Cache(db storage.KeysDB, text []byte, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	uniqKey, err := waitUniqKey(ctx, db)

	if err != nil {
		return "", fmt.Errorf("Error on generating unique key: %w", err)
	}

	err = db.Set(ctx, uniqKey, text)

	if err != nil {
		return "", fmt.Errorf("Error on setting key '%s' in db: %w", uniqKey, err)
	}

	return uniqKey, nil
}

func waitUniqKey(ctx context.Context, db storage.KeysDB) (string, error) {
	keych := make(chan string)
	go sendUniqKey(ctx, db, keych)

	select {
	case key := <-keych:
		return key, nil
	case <-ctx.Done():
		return "", fmt.Errorf("Timeout")
	}
}

func sendUniqKey(ctx context.Context, db storage.KeysDB, keych chan string) {
	key, _ := generateUniqKey(ctx, db)
	keych <- key
}

func generateUniqKey(ctx context.Context, db storage.KeysDB) (string, error) {
	length := 14

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
	for i := 0; i < length; i++ {
		result[i] = chars[r.Intn(len(chars))]
	}

	return string(result)
}
