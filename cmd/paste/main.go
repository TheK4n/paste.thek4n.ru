// Enter point to paste service.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/apikeys"
	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/handlers"
	"github.com/thek4n/paste.thek4n.name/internal/storage"

	flags "github.com/jessevdk/go-flags"
)

var version = "built-from-source"

type options struct {
	Port           int    `short:"p" long:"port" default:"80" description:"Port to listen"`
	Host           string `long:"host" default:"localhost" description:"Host to listen"`
	Health         bool   `long:"health" description:"Enable health handler on /health/ URL"`
	DBPort         int    `long:"dbport" default:"6379" description:"Database port"`
	DBHost         string `long:"dbhost" default:"localhost" description:"Database host"`
	ShowVersion    bool   `short:"v" long:"version" description:"Show version and exit"`
	Logger         string `long:"logger" default:"plain" choice:"json" choice:"plain" description:"Choose type logger"`
	LogLevel       string `long:"loglevel" default:"INFO" choice:"DEBUG" choice:"debug" choice:"INFO" choice:"info" choice:"WARN" choice:"warn" choice:"ERROR" choice:"error" choice:"TRACE" choice:"trace" description:"Logger level"`
	BrokerHost     string `long:"brokerhost" default:"localhost" description:"AMQP broker host"`
	BrokerPort     int    `long:"brokerport" default:"5672" description:"AMQP broker port"`
	BrokerUser     string `long:"brokeruser" default:"guest" description:"AMQP broker user"`
	BrokerPassword string `long:"brokerpassword" default:"guest" description:"AMQP broker password"`
}

func (o *options) getLogLevel() slog.Level {
	levels := map[string]slog.Level{
		"TRACE": config.LevelTrace,
		"DEBUG": slog.LevelDebug,
		"WARN":  slog.LevelWarn,
		"INFO":  slog.LevelInfo,
		"ERROR": slog.LevelError,
	}

	return levels[strings.ToUpper(o.LogLevel)]
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
	handler, err := newLoggerHandler(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cant get logger: %s", err)
		os.Exit(1)
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
		os.Exit(1)
	}
	logger.Info("Connected to database 0 (keys)")

	apikeysDb, err := storage.InitAPIKeysStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	logger.Info("Connected to database 1 (apikeys)")

	quotaDb, err := storage.InitQuotaStorageDB(opts.DBHost, opts.DBPort)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	logger.Info("Connected to database 2 (quota)")

	brokerConnectionURL := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/",
		opts.BrokerUser,
		opts.BrokerPassword,
		getBrokerHost(opts),
		opts.BrokerPort,
	)

	broker, err := apikeys.InitBroker(brokerConnectionURL)
	if err != nil {
		logger.Error("failed to connect to broker", "error", err)
		os.Exit(1)
	}
	logger.Info("Connected to amqp broker")

	handlers := handlers.Application{
		Version:   version,
		DB:        *db,
		APIKeysDB: *apikeysDb,
		QuotaDB:   *quotaDb,
		Broker:    *broker,
		Logger:    *logger,
	}

	mux := newMux(&handlers, opts)

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

func getBrokerHost(opts *options) string {
	brokerHost := os.Getenv("BROKER_HOST")
	if brokerHost == "" {
		return opts.BrokerHost
	}
	return brokerHost
}

func newLoggerHandler(opts *options) (slog.Handler, error) {
	levelNames := map[slog.Leveler]string{
		config.LevelTrace: "TRACE",
	}

	shouldAddSource := opts.getLogLevel() == config.LevelTrace
	handlerOptions := &slog.HandlerOptions{
		Level:     opts.getLogLevel(),
		AddSource: shouldAddSource,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				levelLabel, exists := levelNames[level]
				if !exists {
					levelLabel = level.String()
				}

				a.Value = slog.StringValue(levelLabel)
			}
			return a
		},
	}

	if opts.Logger == "plain" {
		return slog.NewTextHandler(os.Stdout, handlerOptions), nil
	}
	if opts.Logger == "json" {
		return slog.NewJSONHandler(os.Stdout, handlerOptions), nil
	}

	return nil, fmt.Errorf("invalid logger")
}

func newMux(h *handlers.Application, opts *options) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}/{$}", h.Get)
	mux.HandleFunc("GET /{key}/clicks/{$}", h.GetClicks)
	mux.HandleFunc("POST /{$}", h.Cache)
	if opts.Health {
		mux.HandleFunc("GET /health/{$}", h.Healthcheck)
	}

	return mux
}
