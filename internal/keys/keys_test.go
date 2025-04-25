package keys

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

var DB *storage.RecordsDB

func init() {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", "localhost", 6379),
		Password:     "",
		Username:     "",
		DB:           0,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	DB = &storage.RecordsDB{Client: client}
}

func BenchmarkWaitKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateUniqKey(context.Background(), DB)
	}
}
