//go:build e2e

package main

import (
	"context"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
)

type TestQuotaConfig struct{}

func (c TestQuotaConfig) QuotaResetPeriod() time.Duration {
	defconf := config.DefaultQuotaConfig{}
	return defconf.QuotaResetPeriod()
}

func (c TestQuotaConfig) Quota() uint32 {
	return math.MaxUint32
}

type testServer struct {
	*httptest.Server
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

	opts := pasteOptions{
		EnableHealthcheck: true,
		DBHost:            getRedisHost(),
		DBPort:            6379,
	}

	recordsClient := newRedisClient(&opts, 0)
	quotaClient := newRedisClient(&opts, 1)
	apikeyClient := newRedisClient(&opts, 2)

	recordsClient.FlushDB(context.Background())
	quotaClient.FlushDB(context.Background())
	apikeyClient.FlushDB(context.Background())

	handlers := handlersFactory(
		recordsClient,
		quotaClient,
		apikeyClient,
		&opts,
		slog.Default(),
		event.NewPublisher(),
		TestQuotaConfig{},
	)

	mux := http.NewServeMux()
	addHandlers(mux, handlers, &opts)
	return &testServer{
		Server: httptest.NewServer(mux),
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
