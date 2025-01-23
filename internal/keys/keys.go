package keys

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

func Get(ctx context.Context, db storage.KeysDB, key string) ([]byte, error) {
	return db.Get(ctx, key)
}

func Cache(ctx context.Context, db storage.KeysDB, text []byte) (string, error) {
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
	go generateUniqKey(ctx, db, keych)

	select {
	case key := <-keych:
		return key, nil
	case <-ctx.Done():
		return "", fmt.Errorf("Timeout")
	}
}

func generateUniqKey(ctx context.Context, db storage.KeysDB, keych chan string) {
	length := 14

	key := generateKey(length)
	exists, _ := db.Exists(ctx, key)

	for exists {
		select {
		case <-ctx.Done():
			return
		default:
		}

		key = generateKey(length)
		exists, _ = db.Exists(ctx, key)
	}

	keych <- key
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
