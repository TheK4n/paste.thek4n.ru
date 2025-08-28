//go:build e2e

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
)

func TestCache(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("cache returns 200 ok", func(t *testing.T) {
		t.Parallel()

		resp, err := ts.post("/", "test body")
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("cache with expiration time removes key after this time", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		expectedBody := "body"
		resp, err := ts.post("/?ttl=3s", expectedBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		gotUrl := mustReadBody(t, resp.Body)

		resp, err = http.Get(gotUrl)
		require.NoError(t, err)
		require.Equal(t,
			http.StatusOK, resp.StatusCode,
			"shoudn`t be removed yet",
		)

		resp, err = http.Get(gotUrl)
		require.NoError(t, err)
		require.Equal(t,
			http.StatusOK, resp.StatusCode,
			"shoudn`t be removed yet",
		)

		time.Sleep(3500 * time.Millisecond)

		resp, err = http.Get(gotUrl)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("cache request custom key length", func(t *testing.T) {
		t.Parallel()

		const expectedLength = 16
		postResp, err := ts.post(fmt.Sprintf("/?len=%d", expectedLength), "test body")
		require.NoError(t, err)

		gotURL := mustReadBody(t, postResp.Body)
		assert.Equal(t,
			expectedLength, getKeyLength(t, gotURL),
			"key length should match requested length",
		)
	})

	t.Run("unprivileged cache with big body returns 413", func(t *testing.T) {
		t.Parallel()

		largeBody := bytes.Repeat([]byte("a"), int(config.DefaultCacheValidationConfig{}.UnprivilegedMaxBodySize()+100))

		resp, err := ts.post("/", string(largeBody))
		require.NoError(t, err)

		assert.Equal(t,
			http.StatusRequestEntityTooLarge, resp.StatusCode,
			"should reject too large bodies",
		)
	})
}

func TestGet(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("get returns correct body", func(t *testing.T) {
		t.Parallel()

		expectedBody := "test body"
		postResp, err := ts.post("/", expectedBody)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, postResp.StatusCode)
		gotURL := mustReadBody(t, postResp.Body)

		getResp, err := http.Get(gotURL)
		require.NoError(t, err)

		assert.Equal(t,
			expectedBody,
			mustReadBody(t, getResp.Body),
			"retrieved body should match posted body",
		)
	})
}

func TestGetClicks(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("get clicks after zero requests returns zero clicks", func(t *testing.T) {
		t.Parallel()
		postResp, err := ts.post("/", "test body")
		require.NoError(t, err)
		gotURL := mustReadBody(t, postResp.Body)

		clicksResp, err := http.Get(gotURL + "/clicks/")
		require.NoError(t, err)

		assert.Equal(t,
			"0", mustReadBody(t, clicksResp.Body),
			"clicks should be 0 for new paste",
		)
	})

	t.Run("get clicks after number of clicks returns correct clicks", func(t *testing.T) {
		t.Parallel()
		const expectedRequests = 3
		postResp, err := ts.post("/", "test body")
		require.NoError(t, err)
		gotURL := mustReadBody(t, postResp.Body)

		for range expectedRequests {
			_, err := http.Get(gotURL)
			require.NoError(t, err)
		}

		clicksResp, err := http.Get(gotURL + "/clicks/")
		require.NoError(t, err)
		assert.Equal(t,
			strconv.Itoa(expectedRequests), mustReadBody(t, clicksResp.Body),
		)
	})
}

func TestCacheDisposable(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("disposable record will be removed after expected number of get requests", func(t *testing.T) {
		t.Parallel()
		const disposableCount = 3
		postResp, err := ts.post(fmt.Sprintf("/?disposable=%d", disposableCount), "test body")
		require.NoError(t, err)
		gotURL := mustReadBody(t, postResp.Body)

		for range disposableCount {
			resp, err := http.Get(gotURL)
			require.NoError(t, err)
			require.Equal(t,
				http.StatusOK, resp.StatusCode,
				"shoudn`t be removed yet",
			)
		}

		resp, err := http.Get(gotURL)
		require.NoError(t, err)
		assert.Equal(t,
			http.StatusNotFound, resp.StatusCode,
			"should be removed",
		)
	})
}

func TestCacheURL(t *testing.T) {
	ts := setupTestServer(t)

	t.Run("get cached url redirects to expected location", func(t *testing.T) {
		t.Parallel()
		expectedURL := "https://example.com"
		postResp, err := ts.post("/?url=true", expectedURL)
		require.NoError(t, err)
		gotURL := mustReadBody(t, postResp.Body)

		resp, err := noRedirectGet(gotURL)
		require.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
		assert.Equal(t, expectedURL, resp.Header.Get("Location"))
	})
}
