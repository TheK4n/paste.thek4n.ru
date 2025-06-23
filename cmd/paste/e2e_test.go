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

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestCacheSuccess(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	resp, err := ts.post("/", "test body")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK for successful post")
}

func TestGetReturnsCorrectBody(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	expectedBody := "test body"
	postResp, err := ts.post("/", expectedBody)
	require.NoError(t, err)
	defer postResp.Body.Close()

	gotURL := mustReadBody(t, postResp.Body)
	getResp, err := http.Get(gotURL)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, expectedBody, mustReadBody(t, getResp.Body), "retrieved body should match posted body")
}

func TestClicksReturnsZeroAfterZeroRequests(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	postResp, err := ts.post("/", "test body")
	require.NoError(t, err)
	defer postResp.Body.Close()

	gotURL := mustReadBody(t, postResp.Body)
	clicksResp, err := http.Get(gotURL + "/clicks/")
	require.NoError(t, err)
	defer clicksResp.Body.Close()

	assert.Equal(t, "0", mustReadBody(t, clicksResp.Body), "clicks should be 0 for new paste")
}

func TestReturnsCorrectClicksNumberAfterNumberOfRequests(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	const expectedRequests = 3
	postResp, err := ts.post("/", "test body")
	require.NoError(t, err)
	defer postResp.Body.Close()

	gotURL := mustReadBody(t, postResp.Body)

	// Make expectedRequests number of GET requests
	for range expectedRequests {
		resp, err := http.Get(gotURL)
		require.NoError(t, err)
		resp.Body.Close()
	}

	clicksResp, err := http.Get(gotURL + "/clicks/")
	require.NoError(t, err)
	defer clicksResp.Body.Close()

	assert.Equal(t, strconv.Itoa(expectedRequests), mustReadBody(t, clicksResp.Body))
}

func TestUnprivilegedCacheBigBodyReturns413(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	largeBody := bytes.Repeat([]byte("a"), config.UNPREVELEGED_MAX_BODY_SIZE+100)
	resp, err := ts.post("/", string(largeBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, "should reject too large bodies")
}

func TestDisposableRecordRemovesAfterExpectedNumberOfRequests(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	const disposableCount = 2
	postResp, err := ts.post(fmt.Sprintf("/?disposable=%d", disposableCount), "test body")
	require.NoError(t, err)
	defer postResp.Body.Close()

	gotURL := mustReadBody(t, postResp.Body)

	for range disposableCount {
		resp, err := http.Get(gotURL)
		require.NoError(t, err)
		resp.Body.Close()
	}

	// Should be deleted after disposableCount requests
	resp, err := http.Get(gotURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCachedRedirectsToExpectedURL(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	expectedURL := "https://example.com"
	postResp, err := ts.post("/?url=true", expectedURL)
	require.NoError(t, err)
	defer postResp.Body.Close()

	gotURL := mustReadBody(t, postResp.Body)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(gotURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, expectedURL, resp.Header.Get("Location"))
}

func TestRequestCustomKeyLength(t *testing.T) {
	t.Parallel()
	ts := setupTestServer(t)
	defer ts.close(t)

	const expectedLength = 16
	postResp, err := ts.post(fmt.Sprintf("/?len=%d", expectedLength), "test body")
	require.NoError(t, err)
	defer postResp.Body.Close()

	gotURL := mustReadBody(t, postResp.Body)
	assert.Equal(t, expectedLength, getKeyLength(gotURL), "key length should match requested length")
}

func (ts *testServer) post(path, body string) (*http.Response, error) {
	return http.Post(ts.URL+path, http.DetectContentType([]byte(body)), strings.NewReader(body))
}

func (ts *testServer) close(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, ts.db.Client.FlushDB(ctx).Err())
	require.NoError(t, ts.apiKeysDB.Client.FlushDB(ctx).Err())
	require.NoError(t, ts.quotaDB.Client.FlushDB(ctx).Err())
	ts.Server.Close()
}

func mustReadBody(t *testing.T, r io.ReadCloser) string {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

func getKeyLength(url string) int {
	parts := strings.Split(url, "/")
	if len(parts) < 4 {
		return 0
	}
	return len(parts[3])
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	db, err := storage.InitKeysStorageDB(redisHost, 6379)
	require.NoError(t, err, "failed to connect to keys storage")

	apikeysDb, err := storage.InitAPIKeysStorageDB(redisHost, 6379)
	require.NoError(t, err, "failed to connect to api keys storage")

	quotaDb, err := storage.InitQuotaStorageDB(redisHost, 6379)
	require.NoError(t, err, "failed to connect to quota storage")

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
