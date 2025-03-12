package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	flags "github.com/jessevdk/go-flags"
)

type Options struct {
	Simple     bool `long:"simple" default:"false" description:"Just check connection"`
	Http200OK  bool `long:"200" default:"false" description:"Check is status 200"`
	Complex    bool `long:"complex" default:"true" description:"Check server json response availability==true"`
	URL		   string `short:"u" long:"url" required:"true" description:"Target url"`
}


type HealthcheckResponse struct {
	Version      string `json:"version"`
	Availability bool   `json:"availability"`
	Msg          string `json:"msg"`
}

func main() {
	var opts Options
	_, err := flags.Parse(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse params error: %s\n", err)
		os.Exit(2)
	}



	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	defer resp.Body.Close()

	healthCheckResp := HealthcheckResponse{}
	err = json.NewDecoder(resp.Body).Decode(&healthCheckResp)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if !healthCheckResp.Availability {
		fmt.Fprintln(os.Stderr, "Server response:", healthCheckResp.Msg)
		os.Exit(1)
	}

	fmt.Println("Ok")
	os.Exit(0)
}


func simple(url string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Ok")
	os.Exit(0)
}

func simple200() {

}

func complex_() {

}
