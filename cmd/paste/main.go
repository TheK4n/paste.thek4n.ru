// Enter point to paste service.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"

	flags "github.com/jessevdk/go-flags"
)

var version = "built-from-source"

type options struct {
	Port        int    `short:"p" long:"port" default:"80" description:"Port to listen"`
	Host        string `long:"host" default:"localhost" description:"Host to listen"`
	Health      bool   `long:"health" description:"Enable health handler on /health/ URL"`
	DBPort      int    `long:"dbport" default:"6379" description:"Database port"`
	DBHost      string `long:"dbhost" default:"localhost" description:"Database host"`
	ShowVersion bool   `short:"v" long:"version" description:"Show version and exit"`
	Logger      string `long:"logger" default:"plain" choice:"json" choice:"plain" description:"Choose type logger"`
	LogLevel    string `long:"loglevel" default:"INFO" choice:"DEBUG" choice:"debug" choice:"INFO" choice:"info" choice:"WARN" choice:"warn" choice:"ERROR" choice:"error" description:"Logger level"`
}

func main() {
	var opts options
	_, err := flags.Parse(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse params error: %s\n", err)
		os.Exit(2)
	}

	if opts.ShowVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	runServer(&opts)
}

func runServer(opts *options) {
	handler, err := getLoggerHandler(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cant get logger: %s", err)
		return
	}

	logger := slog.New(handler)
	logger.Info("Connecting to database...")

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost != "" {
		opts.DBHost = redisHost
	}

	db, err := storage.InitKeysStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return
	}
	logger.Info("Connected to database 0 (keys)")

	apikeysDb, err := storage.InitAPIKeysStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return
	}
	logger.Info("Connected to database 1 (apikeys)")

	quotaDb, err := storage.InitQuotaStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return
	}
	logger.Info("Connected to database 2 (quota)")

	handlers := handlers.Application{
		Version:   version,
		DB:        *db,
		APIKeysDB: *apikeysDb,
		QuotaDB:   *quotaDb,
		Logger:    *logger,
	}

	mux := getMux(&handlers, opts)

	hostport := fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	server := &http.Server{
		Addr:              hostport,
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           mux,
	}

	logger.Info("Server started", "host", opts.Host, "port", opts.Port)
	err = server.ListenAndServe()
	panic(err)
}

func getLoggerHandler(opts *options) (slog.Handler, error) {
	handlerOptions := &slog.HandlerOptions{
		Level: getLoggerLevel(opts.LogLevel),
	}
	if opts.Logger == "plain" {
		return slog.NewTextHandler(os.Stdout, handlerOptions), nil
	}
	if opts.Logger == "json" {
		return slog.NewJSONHandler(os.Stdout, handlerOptions), nil
	}

	return nil, fmt.Errorf("invalid logger")
}

func getLoggerLevel(level string) slog.Level {
	levels := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"WARN":  slog.LevelWarn,
		"INFO":  slog.LevelInfo,
		"ERROR": slog.LevelError,
	}

	return levels[strings.ToUpper(level)]
}

func getMux(h *handlers.Application, opts *options) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}/{$}", h.Get)
	mux.HandleFunc("GET /{key}/clicks/{$}", h.GetClicks)
	mux.HandleFunc("POST /{$}", h.Cache)
	if opts.Health {
		mux.HandleFunc("GET /health/{$}", h.Healthcheck)
	}

	return mux
}
