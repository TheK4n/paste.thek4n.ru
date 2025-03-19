package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func init() {
	opts := Options{
		Port: 8080,
		Host: "localhost",
		Health: false,
		DBPort: 6379,
		DBHost: "localhost",
	}

	go runServer(&opts)
	time.Sleep(1 * time.Second)
}

func postRequest(url string, body string) (string, error) {
	r := strings.NewReader(body)
	resp, err := http.Post(url, "data/binary", r)
	if err != nil {
		return "", err
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getRequest(url string) (*http.Response, error) {
	return http.Get(url)
}

func getBodyFromResp(resp *http.Response) string {
	buf := new(strings.Builder)
	io.Copy(buf, resp.Body)
	return buf.String()
}

func TestCaching(t *testing.T) {
	body := "hello world."
	generatedLink, err := postRequest("http://localhost:8080/", body)
	if err != nil {
		t.Errorf("Error post request: %s", err)
	}

	response, err := getRequest(generatedLink)
	if err != nil {
		t.Errorf("Error get request: %s", err)
	}

	receivedBody := getBodyFromResp(response)
	if receivedBody != body {
		t.Errorf("Received body '%s' != initial body '%s'\n", receivedBody, body)
	}
}

func TestCachingDisposable(t *testing.T) {
	body := "hello world."
	generatedLink, err := postRequest("http://localhost:8080/?disposable=1", body)
	if err != nil {
		t.Errorf("Error post request: %s", err)
	}

	_, err = getRequest(generatedLink)
	if err != nil {
		t.Errorf("Error get request: %s", err)
	}

	resp, err := getRequest(generatedLink)
	if resp.StatusCode != 404 {
		t.Errorf("Second get request on disposable link returned non-404 code: %d", resp.StatusCode)
	}
}
