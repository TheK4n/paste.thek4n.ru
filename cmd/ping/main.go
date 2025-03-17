package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	flags "github.com/jessevdk/go-flags"
)

type Options struct {
	Method string `long:"method" choice:"simple" choice:"200" choice:"json" default:"simple"`
}

type HealthcheckResponse struct {
	Version      string `json:"version"`
	Availability bool   `json:"availability"`
	Msg          string `json:"msg"`
}

func main() {
	var opts Options
	args, err := flags.Parse(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse params error: %s\n", err)
		os.Exit(2)
	}

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Parse params error: URL argument not provided")
		os.Exit(2)
	}

	switch opts.Method {
	case "simple":
		simpleHealthcheck(args[0])
	case "200":
		simpleHealthcheck200(args[0])
	case "json":
		jsonHealthcheck(args[0])
	}

	os.Exit(0)
}

func jsonHealthcheck(url string) *http.Response {
	resp := simpleHealthcheck200(url)

	healthCheckResp := HealthcheckResponse{}
	err := json.NewDecoder(resp.Body).Decode(&healthCheckResp)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading server answer: ", err)
		os.Exit(1)
	}

	if !healthCheckResp.Availability {
		fmt.Fprintln(os.Stderr, "Server response:", healthCheckResp.Msg)
		os.Exit(1)
	}

	return resp
}

func simpleHealthcheck200(url string) *http.Response {
	resp := simpleHealthcheck(url)

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "Status:", resp.StatusCode)
		os.Exit(1)
	}

	return resp
}

func simpleHealthcheck(url string) *http.Response {
	resp, err := getRequest(url)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	return resp
}

func getRequest(url string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Accept", "application/json")

	return http.DefaultClient.Do(req)
}
