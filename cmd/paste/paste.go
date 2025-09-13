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
	redis "github.com/redis/go-redis/v9"
	"go.uber.org/dig"

	"github.com/thek4n/paste.thek4n.ru/internal/application/service"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/event"
	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/eventhandler"
	"github.com/thek4n/paste.thek4n.ru/internal/infrastructure/repository"
	"github.com/thek4n/paste.thek4n.ru/internal/presentation/webhandlers"
	"github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
)

var version = "built-from-source"

type pasteOptions struct {
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

type digConfig struct {
	dig.In

	Options *pasteOptions
	Logger  *slog.Logger
	Server  *http.Server
}

var mux = http.NewServeMux()

func runServer(args []string) {
	container := buildContainer(args)

	if err := container.Invoke(run); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func buildContainer(args []string) *dig.Container {
	container := dig.New()

	provide := func(constructor any, opts ...dig.ProvideOption) {
		if err := container.Provide(constructor, opts...); err != nil {
			fmt.Fprintf(os.Stderr, "DI Error: %s\n", err)
			os.Exit(1)
		}
	}

	// Provide args
	provide(func() []string { return args })

	// Provide options
	provide(provideOptions)

	// Provide logger components
	provide(provideLoggerHandler)
	provide(provideLogger)

	// Provide Redis clients with names
	provide(provideRecordsClient, dig.Name("records"))
	provide(provideQuotaClient, dig.Name("quota"))
	provide(provideAPIKeyClient, dig.Name("apikey"))

	// Provide AMQP channel
	provide(provideBrokerChannel)

	// Provide event publisher
	provide(provideEventPublisher)

	// Provide repositories
	provide(provideRecordRepository)
	provide(provideAPIKeyRORepository)
	provide(provideQuotaRepository)

	// Provide services
	provide(provideGetService)
	provide(provideAPIKeyService)
	provide(provideCacheService)

	// Provide handlers
	provide(provideHandlers)

	// Provide server
	provide(provideServer)

	return container
}

func provideOptions(args []string) (*pasteOptions, error) {
	var opts pasteOptions

	_, err := flags.NewParser(&opts, flags.Default).ParseArgs(args)
	if err != nil {
		return nil, fmt.Errorf("parse params error: %w", err)
	}

	if opts.ShowVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// Override DBHost if environment variable is set
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		opts.DBHost = redisHost
	}

	return &opts, nil
}

func provideLoggerHandler(opts *pasteOptions) (slog.Handler, error) {
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

func provideLogger(handler slog.Handler) *slog.Logger {
	return slog.New(handler)
}

func provideBrokerChannel(opts *pasteOptions, logger *slog.Logger) (*amqp.Channel, error) {
	brokerHost := os.Getenv("BROKER_HOST")
	if brokerHost == "" {
		brokerHost = opts.BrokerHost
	}

	brokerConnectionURL := fmt.Sprintf(
		"amqp://%s:%s@%s:%d/",
		opts.BrokerUser,
		opts.BrokerPassword,
		brokerHost,
		opts.BrokerPort,
	)

	loggerb := logger.With("broker_host", brokerHost, "broker_port", opts.BrokerPort, "broker_user", opts.BrokerUser)
	loggerb.Debug("Initializing amqp broker channel...")

	brokerChannel, err := initBrokerChannel(brokerConnectionURL, loggerb)
	if err != nil {
		loggerb.Error("Failed to initialize amqp broker channel", "error", err)
		return nil, err
	}

	loggerb.Debug("Successfully initialized amqp broker channel")
	return brokerChannel, nil
}

func provideEventPublisher(brokerChannel *amqp.Channel) *event.Publisher {
	eventPublisher := event.NewPublisher()
	rbmq := eventhandler.NewRabbitMQEventHandler(brokerChannel)
	eventPublisher.Subscribe(rbmq, event.NewAPIKeyUsedEvent("", apikeys.UsageReason_CUSTOMKEY, ""))
	return eventPublisher
}

func provideRecordsClient(opts *pasteOptions) *redis.Client {
	return newRedisClient(opts, 0)
}

func provideQuotaClient(opts *pasteOptions) *redis.Client {
	return newRedisClient(opts, 1)
}

func provideAPIKeyClient(opts *pasteOptions) *redis.Client {
	return newRedisClient(opts, 2)
}

func provideRecordRepository(params struct {
	dig.In
	Client *redis.Client `name:"records"`
},
) *repository.RedisRecordRepository {
	cachingConfig := config.DefaultCachingConfig{}
	return repository.NewRedisRecordRepository(
		params.Client,
		cachingConfig,
	)
}

func provideAPIKeyRORepository(params struct {
	dig.In
	Client *redis.Client `name:"apikey"`
},
) *repository.RedisAPIKeyRORepository {
	return repository.NewRedisAPIKeyRORepository(
		params.Client,
	)
}

func provideQuotaRepository(params struct {
	dig.In
	Client *redis.Client `name:"quota"`
},
) *repository.RedisQuotaRepository {
	quotaConfig := config.DefaultQuotaConfig{}
	return repository.NewRedisQuotaRepository(
		params.Client,
		quotaConfig,
	)
}

func provideGetService(
	recordRepository *repository.RedisRecordRepository,
) *service.GetService {
	return service.NewGetService(recordRepository)
}

func provideAPIKeyService(
	apikeyRORepository *repository.RedisAPIKeyRORepository,
) *service.APIKeyService {
	return service.NewAPIKeyService(apikeyRORepository)
}

func provideCacheService(
	recordRepository *repository.RedisRecordRepository,
	quotaRepository *repository.RedisQuotaRepository,
	apikeyRORepository *repository.RedisAPIKeyRORepository,
	apiKeyService *service.APIKeyService,
	eventPublisher *event.Publisher,
	logger *slog.Logger,
) *service.CacheService {
	cacheValidationConfig := config.DefaultCacheValidationConfig{}
	quotaConfig := config.DefaultQuotaConfig{}

	return service.NewCacheService(
		recordRepository,
		quotaRepository,
		apikeyRORepository,
		apiKeyService,
		eventPublisher,
		cacheValidationConfig,
		quotaConfig,
		logger,
	)
}

func provideHandlers(
	opts *pasteOptions,
	logger *slog.Logger,
	getService *service.GetService,
	cacheService *service.CacheService,
) *webhandlers.Handlers {
	cacheValidationConfig := config.DefaultCacheValidationConfig{}

	return webhandlers.NewHandlers(
		cacheValidationConfig,
		version,
		opts.EnableHealthcheck,
		*logger,
		getService,
		cacheService,
	)
}

func provideServer(
	opts *pasteOptions,
	handlers *webhandlers.Handlers,
) *http.Server {
	mux.HandleFunc("GET /{key}/{$}", handlers.Get)
	mux.HandleFunc("GET /{key}/clicks/{$}", handlers.GetClicks)
	mux.HandleFunc("POST /{$}", handlers.Cache)

	if opts.EnableHealthcheck {
		mux.HandleFunc("GET /health/{$}", handlers.Healthcheck)
	}
	if opts.EnableInteractiveDocs {
		mux.HandleFunc("GET /docs/{$}", handlers.DocsHandler)
		mux.Handle("/docs/static/", handlers.DocsStaticHandler())
	}

	hostport := fmt.Sprintf("%s:%d", opts.Host, opts.Port)

	return &http.Server{
		Addr:              hostport,
		ReadHeaderTimeout: 3 * time.Second,
		Handler:           mux,
	}
}

func run(config digConfig) error {
	serverErrorCh := make(chan error, 1)

	go func() {
		serverErrorCh <- config.Server.ListenAndServe()
	}()

	config.Logger.Info(
		"Server started",
		"host", config.Options.Host,
		"port", config.Options.Port,
	)

	err := <-serverErrorCh
	return fmt.Errorf("server error: %w", err)
}

func (o *pasteOptions) getLogLevel() slog.Level {
	levels := map[string]slog.Level{
		"TRACE": levelTrace,
		"DEBUG": slog.LevelDebug,
		"WARN":  slog.LevelWarn,
		"INFO":  slog.LevelInfo,
		"ERROR": slog.LevelError,
	}

	return levels[strings.ToUpper(o.LogLevel)]
}

func newRedisClient(opts *pasteOptions, db int) *redis.Client {
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
