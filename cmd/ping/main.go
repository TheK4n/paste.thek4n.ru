package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	url := getArgument(1)
	resp, err := http.Get(url)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
	os.Exit(0)
}

func getArgument(n int) string {
	if len(os.Args) < n+1 {
		return "http://localhost:80/ping"
	}

	url := os.Args[n]

	if url == "" {
		return "http://localhost:80/ping"
	}

	return url
}
