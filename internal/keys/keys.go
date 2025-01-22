package keys

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

func Get(db storage.KeysDB, key string) ([]byte, error) {
	return db.Get(key)
}

func Cache(db storage.KeysDB, text []byte) (string, error) {
	uniqKey, err := generateUniqKey(db)
	if err != nil {
		return "", fmt.Errorf("Error on generating unique key: %w", err)
	}

	err = db.Set(uniqKey, text)

	if err != nil {
		return "", fmt.Errorf("Error on setting key '%s' in db: %w", uniqKey, err)
	}

	return uniqKey, nil
}

func generateUniqKey(db storage.KeysDB) (string, error) {
	length := 14

	key := generateKey(length)

	exists, err := db.Exists(key)
	if err != nil {
		return "", err
	}

	if exists {
		return generateUniqKey(db)
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
