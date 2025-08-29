// Ping tool to check health of paste service
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	flags "github.com/jessevdk/go-flags"
)

type pingOptions struct {
	Method string `long:"method" choice:"simple" choice:"200" choice:"json" default:"simple"`
}

type healthcheckResponse struct {
	Version      string `json:"version"`
	Availability bool   `json:"availability"`
	Msg          string `json:"msg"`
}

func pingCommand(args []string) {
	var opts pingOptions

	args, err := flags.NewParser(&opts, flags.Default).ParseArgs(args)
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
		resp := simpleHealthcheck(args[0])
		err = resp.Body.Close()
	case "200":
		resp := simpleHealthcheck200(args[0])
		err = resp.Body.Close()
	case "json":
		resp := jsonHealthcheck(args[0])
		err = resp.Body.Close()
	}

	if err != nil {
		fmt.Printf("Error: %s", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func jsonHealthcheck(url string) *http.Response {
	resp := simpleHealthcheck200(url)

	healthCheckResp := healthcheckResponse{}
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
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	req.Header.Add("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot make request: %s", err)
	}

	return resp, nil
}
