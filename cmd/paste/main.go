package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"

	flags "github.com/jessevdk/go-flags"
)

//go:embed VERSION
var VERSION string

type Options struct {
	Port   int    `short:"p" long:"port" default:"80" description:"Port to listen"`
	Host   string `long:"host" default:"localhost" description:"Host to listen"`
	Health bool   `long:"health" description:"Enable health handler on /health/ URL"`
	DBPort int    `long:"dbport" default:"6379" description:"Database port"`
	DBHost string `long:"dbhost" default:"localhost" description:"Database host"`
}

//go:generate sh -c "echo -n \"$(git describe --tags --abbrev=0 )\" > VERSION"
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

	db, err := storage.InitKeysStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		log.Fatalf("failed to connect to database server: %s\n", err.Error())
		return
	}

	apikeysDb, err := storage.InitAPIKeysStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		log.Fatalf("failed to connect to database server: %s\n", err.Error())
		return
	}

	quotaDb, err := storage.InitQuotaStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		log.Fatalf("failed to connect to database server: %s\n", err.Error())
		return
	}

	handlers := handlers.Application{
		Version:   VERSION,
		DB:        *db,
		ApiKeysDB: *apikeysDb,
		QuotaDB:   *quotaDb,
	}

	mux := getMux(&handlers, opts)

	hostport := fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	log.Printf("Server started on %s ...", hostport)
	log.Fatal(http.ListenAndServe(hostport, mux))
}

func getMux(h *handlers.Application, opts *Options) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}/{$}", h.Get)
	mux.HandleFunc("GET /{key}/clicks/{$}", h.GetClicks)
	mux.HandleFunc("POST /{$}", h.Cache)
	if opts.Health {
		mux.HandleFunc("GET /health/{$}", h.Healthcheck)
	}

	return mux
}
