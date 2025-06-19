//go:build e2e

package main

import (
	"bytes"
	"context"
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
	body := "body"
	bodyReader := strings.NewReader(body)

	resp, err := http.Post(ts.URL+"/", http.DetectContentType([]byte(body)), bodyReader)

	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestGetReturnsCorrectBody(t *testing.T) {
	// Arrange
	ts := setupTestServer(t)
	defer ts.Close()
	body := "body"
	bodyReader := strings.NewReader(body)
	resp, _ := http.Post(ts.URL+"/", http.DetectContentType([]byte(body)), bodyReader)
	gotUrl := readerToString(resp.Body)

	// Act
	resp, err := http.Get(gotUrl)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, body, readerToString(resp.Body))
}

func TestClicksReturnsZeroAfterZeroRequests(t *testing.T) {
	// Arrange
	ts := setupTestServer(t)
	defer ts.Close()
	body := "body"
	bodyReader := strings.NewReader(body)
	resp, _ := http.Post(ts.URL+"/", http.DetectContentType([]byte(body)), bodyReader)
	gotUrl := readerToString(resp.Body)

	// Act
	resp, err := http.Get(gotUrl + "/clicks/")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, "0", readerToString(resp.Body))
}

func TestReturnsCorrectClicksNumberAfterNumberOfRequests(t *testing.T) {
	// Arrange
	ts := setupTestServer(t)
	defer ts.Close()
	body := "body"
	bodyReader := strings.NewReader(body)
	var reqNumber int64 = 3
	clicksResponse, _ := http.Post(ts.URL+"/", http.DetectContentType([]byte(body)), bodyReader)
	gotUrl := readerToString(clicksResponse.Body)

	// Act
	for range reqNumber {
		_, _ = http.Get(gotUrl)
	}
	clicksResponse, err := http.Get(gotUrl + "/clicks/")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, clicksResponse.StatusCode, http.StatusOK)
	assert.Equal(t, strconv.FormatInt(reqNumber, 10), readerToString(clicksResponse.Body))
}

func TestUnprivelegedCacheBigBodyReturns413(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()
	largeBody := bytes.Repeat([]byte("a"), config.UNPREVELEGED_MAX_BODY_SIZE+100)
	bodyReader := bytes.NewReader(largeBody)

	resp, _ := http.Post(ts.URL+"/", http.DetectContentType([]byte(largeBody)), bodyReader)

	assert.Equal(t, resp.StatusCode, http.StatusRequestEntityTooLarge)
}

func readerToString(body io.ReadCloser) string {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(body)
	if err != nil {
		panic(err)
	}
	return buf.String()
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
