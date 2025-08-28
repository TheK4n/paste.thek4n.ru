// Enter point to paste service.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/service"
	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/eventhandler"
	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/repository"
	"github.com/thek4n/paste.thek4n.ru/internal/presentation/webhandlers"
	"github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
)

var version = "built-from-source"

type options struct {
	Port                  int    `short:"p" long:"port" default:"80" description:"Port to listen"`
	Host                  string `long:"host" default:"localhost" description:"Host to listen"`
	EnableHealthcheck     bool   `long:"health" description:"Enable health handler on /health/ URL"`
	DBPort                int    `long:"dbport" default:"6379" description:"Database port"`
	DBHost                string `long:"dbhost" default:"localhost" description:"Database host"`
	ShowVersion           bool   `short:"v" long:"version" description:"Show version and exit"`
	Logger                string `long:"logger" default:"plain" choice:"json" choice:"plain" description:"Choose type logger"`
	LogLevel              string `long:"loglevel" default:"INFO" choice:"DEBUG" choice:"debug" choice:"INFO" choice:"info" choice:"WARN" choice:"warn" choice:"ERROR" choice:"error" choice:"TRACE" choice:"trace" description:"Logger level"`
	BrokerHost            string `long:"brokerhost" default:"localhost" description:"AMQP broker host"`
	BrokerPort            int    `long:"brokerport" default:"5672" description:"AMQP broker port"`
	BrokerUser            string `long:"brokeruser" default:"guest" description:"AMQP broker user"`
	BrokerPassword        string `long:"brokerpassword" default:"guest" description:"AMQP broker password"`
	EnableInteractiveDocs bool   `long:"docs" description:"Enable interactive documentation"`
}

const levelTrace = slog.Level(-8)

var mux = http.NewServeMux()

func (o *options) getLogLevel() slog.Level {
	levels := map[string]slog.Level{
		"TRACE": levelTrace,
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

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost != "" {
		opts.DBHost = redisHost
	}

	brokerConnectionURL := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/",
		opts.BrokerUser,
		opts.BrokerPassword,
		getBrokerHost(opts),
		opts.BrokerPort,
	)

	loggerb := logger.With("broker_host", getBrokerHost(opts), "broker_port", opts.BrokerPort, "broker_user", opts.BrokerUser)
	loggerb.Debug("Initializing amqp broker channel...")
	brokerChannel, err := initBrokerChannel(brokerConnectionURL, loggerb)
	if err != nil {
		loggerb.Error("Failed to initialize amqp broker channel", "error", err)
		os.Exit(1)
	}
	loggerb.Debug("Successfully initialized amqp broker channel")

	eventPublisher := event.NewPublisher()
	rbmq := eventhandler.NewRabbitMQEventHandler(brokerChannel)
	eventPublisher.Subscribe(rbmq, event.NewAPIKeyUsedEvent("", apikeys.UsageReason_CUSTOMKEY, ""))

	handlers := handlersFactory(opts, logger, eventPublisher, config.DefaultQuotaConfig{})
	addHandlers(mux, handlers, opts)

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

func handlersFactory(opts *options, logger *slog.Logger, eventPublisher *event.Publisher, quotaConfig config.QuotaConfig) *webhandlers.Handlers {
	recordsClient := newRedisClient(opts, 0)
	quotaClient := newRedisClient(opts, 1)
	apikeyClient := newRedisClient(opts, 2)

	cachingConfig := config.DefaultCachingConfig{}
	cacheValidationConfig := config.DefaultCacheValidationConfig{}

	redisRecordRepository := repository.NewRedisRecordRepository(
		recordsClient,
		cachingConfig,
	)

	return webhandlers.NewHandlers(
		cacheValidationConfig,
		version,
		opts.EnableHealthcheck,
		*logger,
		service.NewGetService(
			redisRecordRepository,
		),
		service.NewCacheService(
			redisRecordRepository,
			repository.NewRedisQuotaRepository(
				quotaClient,
				quotaConfig,
			),
			repository.NewRedisAPIKeyRORepository(
				apikeyClient,
			),
			eventPublisher,
			cacheValidationConfig,
			quotaConfig,
		),
	)
}

func newRedisClient(opts *options, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", opts.DBHost, opts.DBPort),
		PoolSize:     100,
		Password:     "",
		Username:     "",
		DB:           db,
		MaxRetries:   5,
		DialTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
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
		levelTrace: "TRACE",
	}

	shouldAddSource := opts.getLogLevel() == levelTrace
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

func addHandlers(mux *http.ServeMux, h *webhandlers.Handlers, opts *options) {
	mux.HandleFunc("GET /{key}/{$}", h.Get)
	mux.HandleFunc("GET /{key}/clicks/{$}", h.GetClicks)
	mux.HandleFunc("POST /{$}", h.Cache)

	if opts.EnableHealthcheck {
		mux.HandleFunc("GET /health/{$}", h.Healthcheck)
	}
	if opts.EnableInteractiveDocs {
		mux.HandleFunc("GET /docs/{$}", h.DocsHandler)
		mux.Handle("/docs/static/", h.DocsStaticHandler())
	}
}

func initBrokerChannel(connectURL string, logger *slog.Logger) (*amqp.Channel, error) {
	logger.Debug("Creating amqp connection...")
	rabbitmqcon, err := amqp.Dial(connectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}
	logger.Debug("Successfully created amqp connection")

	logger.Debug("Creating amqp channel...")
	ch, err := rabbitmqcon.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create a rabbitmq channel: %w", err)
	}
	logger.Debug("Successfully created amqp channel")

	logger.Debug("Declaring amqp exchange...", "exchange_type", "topic", "exchange_name", "apikeysusage")
	err = ch.ExchangeDeclare(
		"apikeysusage",
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create a rabbitmq exchange '%s': %w", "apikeysusage", err)
	}
	logger.Debug("Successfully declared amqp exchange...")

	return ch, nil
}
