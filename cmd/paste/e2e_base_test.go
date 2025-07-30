//go:build e2e

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.ru/internal/apikeys"
	"github.com/thek4n/paste.thek4n.ru/internal/handlers"
	"github.com/thek4n/paste.thek4n.ru/internal/storage"
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

	brokerConnectionURL := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/",
		"guest",
		"guest",
		getBrokerHost_(),
		5672,
	)
	broker, err := apikeys.InitBroker(brokerConnectionURL, slog.Default())
	require.NoError(t, err, "failed to connect to broker")

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
		Logger:    *slog.Default(),
		Broker:    *broker,
	}

	return &testServer{
		Server:    httptest.NewServer(newMux(&app, &opts)),
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

func getBrokerHost_() string {
	brokerHost := os.Getenv("BROKER_HOST")
	if brokerHost == "" {
		return "localhost"
	}
	return brokerHost
}
