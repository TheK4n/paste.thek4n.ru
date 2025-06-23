//go:build e2e

package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheWithExpirationTimeRemovesAfterThisTime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	ts := setupTestServer(t)
	defer ts.Close()
	expectedBody := "body"
	bodyReader := strings.NewReader(expectedBody)
	ttl := "3s"
	resp, err := http.Post(fmt.Sprintf("%s/?ttl=%s", ts.URL, ttl), http.DetectContentType([]byte(expectedBody)), bodyReader)
	assert.NoError(t, err)
	gotUrl := mustReadBody(t, resp.Body)

	resp, err = http.Get(gotUrl)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	time.Sleep(3500 * time.Millisecond)

	resp, err = http.Get(gotUrl)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
