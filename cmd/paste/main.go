package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"

	flags "github.com/jessevdk/go-flags"
)

const VERSION = "1.0.0"

type Options struct {
	Port   int    `short:"p" long:"port" default:"80" description:"Port to listen"`
	Host   string `long:"host" default:"localhost" description:"Host to listen"`
	Health bool   `long:"health" description:"Enable health handler on /health/ URL"`
	DBPort int    `long:"dbport" default:"6379" description:"Database port"`
	DBHost string `long:"dbhost" default:"localhost" description:"Database host"`
}

func main() {
	var opts Options
	_, err := flags.Parse(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse params error: %s\n", err)
		os.Exit(2)
	}

	runServer(&opts)
}

func runServer(opts *Options) {
	log.Println("Connecting to database...")

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost != "" {
		opts.DBHost = redisHost
	}

	db, err := storage.InitStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		log.Fatalf("failed to connect to database server: %s\n", err.Error())
		return
	}

	handlers := handlers.Application{Version: VERSION, Db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}/", handlers.Get)
	mux.HandleFunc("POST /", handlers.Cache)
	if opts.Health {
		mux.HandleFunc("GET /health/", handlers.Healthcheck)
	}

	hostport := fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	log.Printf("Server started on %s ...", hostport)
	log.Fatal(http.ListenAndServe(hostport, mux))
}
