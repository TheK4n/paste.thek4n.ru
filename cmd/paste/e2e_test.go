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
	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

func TestCacheSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	expectedBody := "body"
	bodyReader := strings.NewReader(expectedBody)

	resp, err := http.Post(ts.URL+"/", http.DetectContentType([]byte(expectedBody)), bodyReader)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetReturnsCorrectBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	expectedBody := "body"
	bodyReader := strings.NewReader(expectedBody)
	response, err := http.Post(ts.URL+"/", http.DetectContentType([]byte(expectedBody)), bodyReader)
	assert.NoError(t, err)
	gotUrl := readerToString(response.Body)

	response, err = http.Get(gotUrl)

	assert.NoError(t, err)
	assert.Equal(t, expectedBody, readerToString(response.Body))
}

func TestClicksReturnsZeroAfterZeroRequests(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	expectedBody := "body"
	bodyReader := strings.NewReader(expectedBody)
	response, _ := http.Post(ts.URL+"/", http.DetectContentType([]byte(expectedBody)), bodyReader)
	gotUrl := readerToString(response.Body)

	response, err := http.Get(gotUrl + "/clicks/")

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "0", readerToString(response.Body))
}

func TestReturnsCorrectClicksNumberAfterNumberOfRequests(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	expectedBody := "body"
	bodyReader := strings.NewReader(expectedBody)
	var expectedRequestsNumber int64 = 3
	clicksResponse, _ := http.Post(ts.URL+"/", http.DetectContentType([]byte(expectedBody)), bodyReader)
	gotUrl := readerToString(clicksResponse.Body)

	for range expectedRequestsNumber {
		_, _ = http.Get(gotUrl)
	}
	clicksResponse, err := http.Get(gotUrl + "/clicks/")

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, clicksResponse.StatusCode)
	assert.Equal(t, formatInt10(expectedRequestsNumber), readerToString(clicksResponse.Body))
}

func TestUnprivelegedCacheBigBodyReturns413(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	largeBody := bytes.Repeat([]byte("a"), config.UNPREVELEGED_MAX_BODY_SIZE+100)
	bodyReader := bytes.NewReader(largeBody)

	resp, _ := http.Post(ts.URL+"/", http.DetectContentType([]byte(largeBody)), bodyReader)

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}

func TestDisposableRecordRemovesAfterExpectedNumberOfRequests(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	expectedBody := "body"
	bodyReader := strings.NewReader(expectedBody)
	disposableCount := 2
	resp, err := http.Post(fmt.Sprintf("%s/?disposable=%d", ts.URL, disposableCount), http.DetectContentType([]byte(expectedBody)), bodyReader)
	assert.NoError(t, err)
	gotUrl := readerToString(resp.Body)

	for range disposableCount {
		_, err = http.Get(gotUrl)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
	resp, err = http.Get(gotUrl)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCachedRedirectsToExpectedURL(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	expectedURL := "https://example.com"
	bodyReader := bytes.NewReader([]byte(expectedURL))
	resp, err := http.Post(fmt.Sprintf("%s/?url=true", ts.URL), http.DetectContentType([]byte(expectedURL)), bodyReader)
	assert.NoError(t, err)
	gotUrl := readerToString(resp.Body)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err = client.Get(gotUrl)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, expectedURL, resp.Header.Get("Location"))
}

func TestRequestCustomKeyLength(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	body := "body"
	bodyReader := strings.NewReader(body)
	expectedLength := 16

	resp, err := http.Post(fmt.Sprintf("%s/?len=%d", ts.URL, expectedLength), http.DetectContentType([]byte(body)), bodyReader)

	assert.NoError(t, err)
	assert.Equal(t, expectedLength, getLenOfUrlKey(readerToString(resp.Body)))
}

func getLenOfUrlKey(url string) int {
	key := strings.Split(url, "/")[3]
	return len(key)
}

func readerToString(body io.ReadCloser) string {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(body)
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func formatInt10(i int64) string {
	return strconv.FormatInt(i, 10)
}

func setupTestServer(t *testing.T) *httptest.Server {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	db, err := storage.InitKeysStorageDB(redisHost, 6379)
	if err != nil {
		t.Fatalf("Failed to connect to database server: %v", err)
	}

	apikeysDb, err := storage.InitAPIKeysStorageDB(redisHost, 6379)
	if err != nil {
		t.Fatalf("Failed to connect to database server: %v", err)
	}

	quotaDb, err := storage.InitQuotaStorageDB(redisHost, 6379)
	if err != nil {
		t.Fatalf("Failed to connect to database server: %v", err)
	}

	opts := Options{
		Health: true,
	}

	app := handlers.Application{
		Version:   "test",
		DB:        *db,
		ApiKeysDB: *apikeysDb,
		QuotaDB:   *quotaDb,
	}
	ts := httptest.NewServer(getMux(&app, &opts))

	err = db.Client.FlushDB(context.Background()).Err()
	if err != nil {
		t.Fatalf("Failed to flush db: %v", err)
	}
	err = apikeysDb.Client.FlushDB(context.Background()).Err()
	if err != nil {
		t.Fatalf("Failed to flush db: %v", err)
	}
	err = quotaDb.Client.FlushDB(context.Background()).Err()
	if err != nil {
		t.Fatalf("Failed to flush db: %v", err)
	}

	return ts
}
