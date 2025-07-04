//go:build e2e

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

type testServer struct {
	*httptest.Server
	db        *storage.KeysDB
	apiKeysDB *storage.APIKeysDB
	quotaDB   *storage.QuotaDB
}

func (ts *testServer) post(path, body string) (*http.Response, error) {
	return http.Post(
		ts.URL+path,
		http.DetectContentType([]byte(body)),
		strings.NewReader(body),
	)
}

func noRedirectGet(url string) (*http.Response, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client.Get(url)
}

func mustReadBody(t *testing.T, r io.ReadCloser) string {
	t.Helper()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	return string(b)
}

func getKeyLength(t *testing.T, url string) int {
	t.Helper()
	parts := strings.Split(url, "/")
	require.True(t, len(parts) >= 4, "Invalid url")
	return len(parts[3])
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	redisHost := getRedisHost()
	redisPort := 6379

	db, err := storage.InitKeysStorageDB(redisHost, redisPort)
	require.NoError(t, err, "failed to connect to keys storage")

	apikeysDb, err := storage.InitAPIKeysStorageDB(redisHost, redisPort)
	require.NoError(t, err, "failed to connect to api keys storage")

	quotaDb, err := storage.InitQuotaStorageDB(redisHost, redisPort)
	require.NoError(t, err, "failed to connect to quota storage")

	ctx := context.Background()
	require.NoError(t, db.Client.FlushDB(ctx).Err())
	require.NoError(t, apikeysDb.Client.FlushDB(ctx).Err())
	require.NoError(t, quotaDb.Client.FlushDB(ctx).Err())

	opts := options{Health: true}

	app := handlers.Application{
		Version:   "test",
		DB:        *db,
		APIKeysDB: *apikeysDb,
		QuotaDB:   *quotaDb,
	}

	return &testServer{
		Server:    httptest.NewServer(getMux(&app, &opts)),
		db:        db,
		apiKeysDB: apikeysDb,
		quotaDB:   quotaDb,
	}
}

func getRedisHost() string {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		return "localhost"
	}
	return redisHost
}
