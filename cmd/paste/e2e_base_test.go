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

	amqp "github.com/rabbitmq/amqp091-go"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/dig"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/presentation/webhandlers"
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
	container *dig.Container
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

func provideTestOptions() (*pasteOptions, error) {
	opts := &pasteOptions{
		Port:              0,
		Host:              "localhost",
		EnableHealthcheck: true,
		DBPort:            6379,
		DBHost:            getRedisHost(),
		Logger:            "plain",
		LogLevel:          "INFO",
	}
	return opts, nil
}

func provideTestQuotaConfig() config.QuotaConfig {
	return TestQuotaConfig{}
}

func provideTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func buildTestContainer() *dig.Container {
	container := dig.New()

	provide := func(constructor interface{}, opts ...dig.ProvideOption) {
		if err := container.Provide(constructor, opts...); err != nil {
			panic(err)
		}
	}

	// Provide test options
	provide(provideTestOptions)

	// Provide test logger
	provide(provideTestLogger)

	// Provide Redis clients with names
	provide(provideRecordsClient, dig.Name("records"))
	provide(provideQuotaClient, dig.Name("quota"))
	provide(provideAPIKeyClient, dig.Name("apikey"))

	provide(provideTestBrokerChannel)

	// Provide event publisher
	provide(provideEventPublisher)

	// Provide repositories
	provide(provideRecordRepository)
	provide(provideAPIKeyRORepository)
	provide(provideQuotaRepository)

	// Provide services
	provide(provideGetService)
	provide(provideAPIKeyService)
	provide(provideCacheService)

	// Provide handlers
	provide(provideHandlers)

	// Provide server with test modifications
	provide(provideTestServer)

	return container
}

func provideTestBrokerChannel() (*amqp.Channel, error) {
	return nil, nil
}

func provideTestServer(
	opts *pasteOptions,
	handlers *webhandlers.Handlers,
) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{key}/{$}", handlers.Get)
	mux.HandleFunc("GET /{key}/clicks/{$}", handlers.GetClicks)
	mux.HandleFunc("POST /{$}", handlers.Cache)

	if opts.EnableHealthcheck {
		mux.HandleFunc("GET /health/{$}", handlers.Healthcheck)
	}

	// Используем случайный порт для тестов
	return &http.Server{
		Addr:              "localhost:0",
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           mux,
	}
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	container := buildTestContainer()

	var recordsClient, quotaClient, apikeyClient *redis.Client
	err := container.Invoke(func(
		rc struct {
			dig.In
			Client *redis.Client `name:"records"`
		},
		qc struct {
			dig.In
			Client *redis.Client `name:"quota"`
		},
		ac struct {
			dig.In
			Client *redis.Client `name:"apikey"`
		},
		opts *pasteOptions,
	) {
		recordsClient = rc.Client
		quotaClient = qc.Client
		apikeyClient = ac.Client

		recordsClient.Options().DB = 10
		quotaClient.Options().DB = 11
		apikeyClient.Options().DB = 12

		ctx := context.Background()
		recordsClient.FlushDB(ctx)
		quotaClient.FlushDB(ctx)
		apikeyClient.FlushDB(ctx)
	})
	require.NoError(t, err)

	var testSrv *testServer
	err = container.Invoke(func(server *http.Server) {
		ts := httptest.NewServer(server.Handler)
		testSrv = &testServer{
			Server:    ts,
			container: container,
		}
	})
	require.NoError(t, err)

	return testSrv
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
