//go:build e2e

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

type testServer struct {
	*httptest.Server
	db        *storage.KeysDB
	apiKeysDB *storage.APIKeysDB
	quotaDB   *storage.QuotaDB
}

func TestCache(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("cache returns 200 ok", func(t *testing.T) {
		resp, err := ts.post("/", "test body")
		require.NoError(t, err)

		assert.Equal(t,
			http.StatusOK, resp.StatusCode,
			"should return 200 OK for successful post",
		)
	})

	t.Run("cache with expiration time removes key after this time", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		expectedBody := "body"
		resp, err := ts.post("/?ttl=3s", expectedBody)
		require.NoError(t, err)
		gotUrl := mustReadBody(t, resp.Body)

		resp, err = http.Get(gotUrl)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		time.Sleep(3500 * time.Millisecond)

		resp, err = http.Get(gotUrl)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("cache request custom key length", func(t *testing.T) {
		const expectedLength = 16
		postResp, err := ts.post(fmt.Sprintf("/?len=%d", expectedLength), "test body")
		require.NoError(t, err)

		gotURL := mustReadBody(t, postResp.Body)
		assert.Equal(t,
			expectedLength, getKeyLength(t, gotURL),
			"key length should match requested length",
		)
	})

	t.Run("unpriveleged cache with big body returns 413", func(t *testing.T) {
		largeBody := bytes.Repeat([]byte("a"), config.UNPREVELEGED_MAX_BODY_SIZE+100)

		resp, err := ts.post("/", string(largeBody))
		require.NoError(t, err)

		assert.Equal(t,
			http.StatusRequestEntityTooLarge, resp.StatusCode,
			"should reject too large bodies",
		)
	})
}

func TestGetReturnsCorrectBody(t *testing.T) {
	ts := setupTestServer(t)

	expectedBody := "test body"
	postResp, err := ts.post("/", expectedBody)
	require.NoError(t, err)

	gotURL := mustReadBody(t, postResp.Body)
	getResp, err := http.Get(gotURL)
	require.NoError(t, err)

	assert.Equal(t, expectedBody, mustReadBody(t, getResp.Body), "retrieved body should match posted body")
}

func TestClicksReturnsZeroAfterZeroRequests(t *testing.T) {
	ts := setupTestServer(t)

	postResp, err := ts.post("/", "test body")
	require.NoError(t, err)

	gotURL := mustReadBody(t, postResp.Body)
	clicksResp, err := http.Get(gotURL + "/clicks/")
	require.NoError(t, err)

	assert.Equal(t, "0", mustReadBody(t, clicksResp.Body), "clicks should be 0 for new paste")
}

func TestReturnsCorrectClicksNumberAfterNumberOfRequests(t *testing.T) {
	ts := setupTestServer(t)

	const expectedRequests = 3
	postResp, err := ts.post("/", "test body")
	require.NoError(t, err)

	gotURL := mustReadBody(t, postResp.Body)

	// Make expectedRequests number of GET requests
	for range expectedRequests {
		_, err := http.Get(gotURL)
		require.NoError(t, err)
	}

	clicksResp, err := http.Get(gotURL + "/clicks/")
	require.NoError(t, err)

	assert.Equal(t, strconv.Itoa(expectedRequests), mustReadBody(t, clicksResp.Body))
}

func TestDisposableRecordRemovesAfterExpectedNumberOfRequests(t *testing.T) {
	ts := setupTestServer(t)

	const disposableCount = 2
	postResp, err := ts.post(fmt.Sprintf("/?disposable=%d", disposableCount), "test body")
	require.NoError(t, err)

	gotURL := mustReadBody(t, postResp.Body)

	for range disposableCount {
		_, err := http.Get(gotURL)
		require.NoError(t, err)
	}

	// Should be deleted after disposableCount requests
	resp, err := http.Get(gotURL)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCachedRedirectsToExpectedURL(t *testing.T) {
	ts := setupTestServer(t)

	expectedURL := "https://example.com"
	postResp, err := ts.post("/?url=true", expectedURL)
	require.NoError(t, err)

	gotURL := mustReadBody(t, postResp.Body)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(gotURL)
	require.NoError(t, err)

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, expectedURL, resp.Header.Get("Location"))
}

func (ts *testServer) post(path, body string) (*http.Response, error) {
	return http.Post(ts.URL+path, http.DetectContentType([]byte(body)), strings.NewReader(body))
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

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
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

	opts := Options{Health: true}

	app := handlers.Application{
		Version:   "test",
		DB:        *db,
		ApiKeysDB: *apikeysDb,
		QuotaDB:   *quotaDb,
	}

	return &testServer{
		Server:    httptest.NewServer(getMux(&app, &opts)),
		db:        db,
		apiKeysDB: apikeysDb,
		quotaDB:   quotaDb,
	}
}
