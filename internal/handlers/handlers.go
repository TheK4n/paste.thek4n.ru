// Package handlers provides handlers
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thek4n/paste.thek4n.name/internal/apikeys"
	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
	apikeysm "github.com/thek4n/paste.thek4n.name/pkg/apikeys"
)

const redirectBody = `<html><head>
<title>303 See Other</title>
</head><body>
<h1>See Other</h1>
<p>The document has moved <a href="%s">here</a>.</p>
</body></html>`

// Application struct contains databases connections.
type Application struct {
	Version   string
	DB        storage.KeysDB
	APIKeysDB storage.APIKeysDB
	QuotaDB   storage.QuotaDB
	Broker    apikeys.Broker
	Logger    slog.Logger
}

type healthcheckResponse struct {
	Version      string `json:"version"`
	Availability bool   `json:"availability"`
	Msg          string `json:"msg"`
}

// Healthcheck checks database availability and returns version.
func (app *Application) Healthcheck(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	resp := &healthcheckResponse{
		Version:      app.Version,
		Availability: true,
		Msg:          "ok",
	}
	statusCode := http.StatusOK

	ctx, cancel := context.WithTimeout(context.Background(), config.HealthcheckTimeout)
	defer cancel()
	if !app.checkIsDatabaseAvailable(ctx) {
		resp.Availability = false
		resp.Msg = "Error connection to database"
		statusCode = http.StatusServiceUnavailable
	}

	if err := sendJSONResponse(w, resp, statusCode); err != nil {
		app.Logger.Error(
			"Error on answer healthcheck",
			"error", err,
			"source_ip", remoteAddr,
			"answer_code", statusCode,
		)
		return
	}
}

func (app *Application) checkIsDatabaseAvailable(ctx context.Context) bool {
	return app.DB.Ping(ctx)
}

func sendJSONResponse(
	w http.ResponseWriter,
	data any,
	statusCode int,
) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}
	return nil
}

type cacheRequest struct {
	APIKey       string
	TTL          time.Duration
	Length       int
	Disposable   int
	IsURL        bool
	RequestedKey string
	Body         []byte
	Authorized   bool
	APIKeyID     string
}

// Cache handle to save key in db.
func (app *Application) Cache(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	requestUUID := uuid.NewString()
	logger := app.Logger.With("source_ip", remoteAddr, "request_id", requestUUID)
	logger.Debug("Start caching key")

	cacheReq, err := app.processCacheRequest(r, logger)
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if cacheReq.Authorized {
		logger = logger.With("authorized", true)
		logger.Debug("Authorize apikey", "apikey_id", cacheReq.APIKeyID)
	}

	if !cacheReq.Authorized {
		if err := app.checkQuota(remoteAddr, logger); err != nil {
			handleCacheError(w, err, logger)
			return
		}
	}

	if err := app.validateCacheRequest(cacheReq); err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if cacheReq.Authorized {
		app.logAPIKeyUsage(cacheReq.APIKeyID, remoteAddr, cacheReq, logger)
	}

	key, err := app.saveKey(cacheReq)
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if err := sendSuccessResponse(w, r, key); err != nil {
		handleCacheError(w, err, logger)
		return
	}

	logger.Info("Set key",
		"key", key,
		"body_size", len(cacheReq.Body),
		"ttl", cacheReq.TTL,
		"disposable", cacheReq.Disposable,
		"isURL", cacheReq.IsURL,
	)
}

func (app *Application) processCacheRequest(r *http.Request, logger *slog.Logger) (*cacheRequest, error) {
	urlQuery := r.URL.Query()
	req := &cacheRequest{}

	req.APIKey = urlQuery.Get("apikey")
	if req.APIKey != "" {
		authorized, err := app.validateApikey(req.APIKey)
		if err != nil {
			logger.Error("fail to check apikey", "error", err, "answer_code", http.StatusInternalServerError)
			return nil, &cacheError{Message: "Failed to check apikey", StatusCode: http.StatusInternalServerError, Err: err}
		}
		req.Authorized = authorized

		if authorized {
			apiKeyID, err := app.getAPIKeyID(req.APIKey)
			if err != nil {
				logger.Warn("fail to fetch apikey id")
			}
			req.APIKeyID = apiKeyID
		}
	}

	if err := readRequestBody(r, req); err != nil {
		return nil, err
	}

	if err := app.parseRequestParams(urlQuery, req); err != nil {
		return nil, err
	}

	return req, nil
}

func readRequestBody(r *http.Request, req *cacheRequest) error {
	maxBodySize := config.UnprevelegedMaxBodySize
	if req.Authorized {
		maxBodySize = config.PrevelegedMaxBodySize
	}

	if r.ContentLength > maxBodySize {
		return &cacheError{
			Message:    fmt.Sprintf("Body too large. Maximum is %d bytes", maxBodySize),
			StatusCode: http.StatusRequestEntityTooLarge,
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil && err != io.EOF {
		return &cacheError{
			Message:    "Failed to read body",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	req.Body = body

	return nil
}

func (app *Application) parseRequestParams(urlQuery url.Values, req *cacheRequest) error {
	var err error

	req.TTL, err = getTTL(urlQuery)
	if err != nil {
		return &cacheError{Message: "Invalid 'ttl' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	req.Length, err = getLength(urlQuery)
	if err != nil {
		return &cacheError{Message: "Invalid 'len' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	req.Disposable, err = getDisposable(urlQuery)
	if err != nil {
		return &cacheError{Message: "Invalid 'disposable' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	req.IsURL, err = getURL(urlQuery)
	if err != nil {
		return &cacheError{Message: "Invalid 'url' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	req.RequestedKey, err = getRequestedKey(urlQuery)
	if err != nil {
		return &cacheError{Message: err.Error(), StatusCode: http.StatusUnprocessableEntity}
	}

	return nil
}

func (app *Application) checkQuota(remoteAddr string, logger *slog.Logger) error {
	quotaValid, err := app.QuotaDB.IsQuotaValid(context.Background(), remoteAddr)
	if err != nil {
		return &cacheError{
			Message:    "Failed to check quota",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}
	if !quotaValid {
		return &cacheError{
			Message:    "Your quota for today is exhausted",
			StatusCode: http.StatusUnauthorized,
		}
	}

	if err := app.QuotaDB.ReduceQuota(context.Background(), remoteAddr, logger); err != nil {
		return &cacheError{
			Message:    "Failed to reduce quota",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}

	return nil
}

func (app *Application) validateCacheRequest(req *cacheRequest) error {
	if req.TTL == 0 && !req.Authorized {
		return &cacheError{
			Message:    "Unauthorized attempt to set persist key",
			StatusCode: http.StatusUnauthorized,
		}
	}

	if req.Length < config.UnprivelegedMinKeyLength && !req.Authorized {
		return &cacheError{
			Message:    "Unauthorized attempt to set short key",
			StatusCode: http.StatusUnauthorized,
		}
	}

	if req.RequestedKey != "" && !req.Authorized {
		return &cacheError{
			Message:    "Unauthorized attempt to set custom key",
			StatusCode: http.StatusUnauthorized,
		}
	}

	if req.IsURL {
		req.Body = []byte(strings.TrimSpace(string(req.Body)))
		if !validateURL(string(req.Body)) {
			return &cacheError{
				Message:    "Invalid 'url' parameter",
				StatusCode: http.StatusUnprocessableEntity,
			}
		}
	}

	return nil
}

// logAPIKeyUsage логирует использование API ключа.
func (app *Application) logAPIKeyUsage(apiKeyID, remoteAddr string, req *cacheRequest, logger *slog.Logger) {
	var reason apikeysm.UsageReason
	switch {
	case req.TTL == 0:
		reason = apikeysm.UsageReason_PERSISTKEY
	case req.Length < config.UnprivelegedMinKeyLength:
		reason = apikeysm.UsageReason_CUSTOMKEYLEN
	case req.RequestedKey != "":
		reason = apikeysm.UsageReason_CUSTOMKEY
	case len(req.Body) > int(config.UnprevelegedMaxBodySize):
		reason = apikeysm.UsageReason_LARGEBODY
	default:
		return
	}

	if err := app.Broker.SendAPIKeyUsageLog(apiKeyID, reason, remoteAddr); err != nil {
		logger.Warn("Fail to publish to broker", "error", err)
	}
	logger.Debug("Sent apikey usage reason to broker", "reason", reason)
}

func (app *Application) saveKey(req *cacheRequest) (string, error) {
	record := storage.KeyRecord{
		Body:       req.Body,
		Disposable: req.Disposable != 0,
		Countdown:  req.Disposable,
		URL:        req.IsURL,
		Clicks:     0,
	}

	var key string
	var err error

	if req.RequestedKey == "" {
		key, err = keys.CacheGeneratedKey(app.DB, 4*time.Second, req.TTL, req.Length, record)
	} else {
		key, err = keys.CacheRequestedKey(app.DB, 4*time.Second, req.RequestedKey, req.TTL, record)
		if errors.Is(err, keys.ErrKeyAlreadyTaken) {
			return "", &cacheError{
				Message:    "Key already taken",
				StatusCode: http.StatusConflict,
			}
		}
	}

	if err != nil {
		return "", &cacheError{
			Message:    "Failed to set key",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}

	return key, nil
}

func sendSuccessResponse(w http.ResponseWriter, r *http.Request, key string) error {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	proto := detectProto(r)
	if _, err := fmt.Fprintf(w, "%s://%s/%s/", proto, r.Host, key); err != nil {
		return &cacheError{
			Message:    "Failed to send response",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}

	return nil
}

func handleCacheError(w http.ResponseWriter, err error, logger *slog.Logger) {
	cacheErr, ok := err.(*cacheError)
	if !ok {
		cacheErr = &cacheError{
			Message:    "Internal server error",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}

	logger.Error(cacheErr.Message,
		"error", cacheErr.Err,
		"answer_code", cacheErr.StatusCode,
	)

	w.WriteHeader(cacheErr.StatusCode)
	if cacheErr.Message != "" {
		_, _ = fmt.Fprint(w, cacheErr.Message)
	}
}

type cacheError struct {
	Message    string
	StatusCode int
	Err        error
}

func (e *cacheError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Get handle getting key.
func (app *Application) Get(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	requestUUID := uuid.NewString()

	logger := app.Logger.With(
		"source_ip", remoteAddr,
		"request_id", requestUUID,
	)

	logger.Debug(
		"Start getting key",
	)

	key := r.PathValue("key")

	logger = logger.With(
		"key", key,
	)

	record, getKeyErr := keys.Get(app.DB, key, 4*time.Second)

	if getKeyErr != nil {
		if getKeyErr == storage.ErrKeyNotFound || errors.Unwrap(getKeyErr) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := w.Write([]byte("404 Not Found"))
			if writeErr != nil {
				logger.Error(
					"Fail to answer",
					"error", writeErr,
					"answer_code", http.StatusInternalServerError,
				)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			return
		}
		logger.Error(
			"Fail to get key",
			"error", getKeyErr,
			"answer_code", http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if record.URL {
		answer := make([]byte, 0)
		answer = fmt.Appendf(answer, redirectBody, string(record.Body))
		w.Header().Set("content-type", http.DetectContentType(answer))
		http.Redirect(w, r, strings.TrimSpace(string(record.Body)), http.StatusSeeOther)
		_, writeErr := w.Write(answer)
		if writeErr != nil {
			logger.Error(
				"Fail to answer",
				"error", writeErr,
				"answer_code", http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		logger.Info(
			"Redirect url",
		)
		return
	}

	w.Header().Set("content-type", http.DetectContentType(record.Body))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(record.Body)
	if writeErr != nil {
		logger.Error(
			"Fail to answer",
			"error", writeErr,
			"answer_code", http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger.Info(
		"Get content",
	)
}

// GetClicks handle getting clicks for key request.
func (app *Application) GetClicks(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	requestUUID := uuid.NewString()

	logger := app.Logger.With(
		"source_ip", remoteAddr,
		"request_id", requestUUID,
	)

	logger.Debug(
		"Start getting key clicks",
	)

	key := r.PathValue("key")

	logger = logger.With(
		"key", key,
	)

	clicks, err := keys.GetClicks(app.DB, key, 4*time.Second)
	if err != nil {
		if err == storage.ErrKeyNotFound || errors.Unwrap(err) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := fmt.Fprint(w, "404 Not Found")
			if writeErr != nil {
				logger.Error(
					"Fail to answer",
					"error", writeErr,
					"answer_code", http.StatusInternalServerError,
				)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			return
		}
		logger.Error(
			"Fail to get key clicks",
			"error", err,
			"answer_code", http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body := []byte(strconv.Itoa(clicks))
	w.Header().Set("content-type", http.DetectContentType(body))
	w.WriteHeader(http.StatusOK)

	_, writeErr := w.Write(body)
	if writeErr != nil {
		logger.Error(
			"Fail to answer",
			"error", writeErr,
			"answer_code", http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger.Info(
		"Get clicks",
	)
}

func getTTL(v url.Values) (time.Duration, error) {
	ttlQuery := v.Get("ttl")

	if ttlQuery == "" {
		return config.DefaultTTL, nil
	}

	ttl, err := time.ParseDuration(ttlQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse duration: %w", err)
	}

	if ttl < config.MinTTL {
		return 0, fmt.Errorf("TTL can`t be less then %s", config.MinTTL)
	}

	if ttl > config.MaxTTL {
		return 0, fmt.Errorf("TTL can`t be more then %s", config.MaxTTL)
	}

	return ttl, nil
}

func getRequestedKey(v url.Values) (string, error) {
	requestedKey := v.Get("key")

	if requestedKey == "" {
		return "", nil
	}

	if len(requestedKey) > config.MaxKeyLength {
		return "", fmt.Errorf("requested key length more then max")
	}

	if len(requestedKey) < config.PrivelegedMinKeyLength {
		return "", fmt.Errorf("requested key length less then min")
	}

	for _, char := range requestedKey {
		if !strings.ContainsRune(config.Charset, char) {
			return "", fmt.Errorf("requested key contains illegal char")
		}
	}

	return requestedKey, nil
}

func getDisposable(v url.Values) (int, error) {
	disposableQuery := v.Get("disposable")

	if disposableQuery == "" {
		return 0, nil
	}

	disposable, err := strconv.Atoi(disposableQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse disposable: %w", err)
	}

	if disposable < 0 {
		return 0, fmt.Errorf("disposable argument can`t be less then zero")
	}

	if disposable > 255 {
		return 0, fmt.Errorf("disposable argument can`t be more then 255")
	}

	return disposable, nil
}

func getLength(v url.Values) (int, error) {
	lengthQuery := v.Get("len")

	if lengthQuery == "" {
		return config.DefaultKeyLength, nil
	}

	length, err := strconv.Atoi(lengthQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse length: %w", err)
	}

	if length < config.PrivelegedMinKeyLength {
		return 0, fmt.Errorf("length can`t be less then %d", config.PrivelegedMinKeyLength)
	}

	if length > config.MaxKeyLength {
		return 0, fmt.Errorf("length can`t be more then %d", config.MaxKeyLength)
	}

	return length, nil
}

func getURL(v url.Values) (bool, error) {
	URLQuery := v.Get("url")

	if URLQuery == "" {
		return false, nil
	}

	if URLQuery == "true" {
		return true, nil
	}

	if URLQuery == "false" {
		return false, nil
	}

	return false, fmt.Errorf("URL argument can be only 'true' or 'false'")
}

func detectProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}

	proto := r.Header.Get("X-Forwarded-Proto")
	if proto != "" {
		return proto
	}

	return "http"
}

func validateURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func (app *Application) validateApikey(str string) (bool, error) {
	record, err := app.APIKeysDB.Get(context.Background(), str)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("fail to get api key: %w", err)
	}

	return record.Valid, nil
}

func (app *Application) getAPIKeyID(str string) (string, error) {
	record, err := app.APIKeysDB.Get(context.Background(), str)
	if err != nil {
		return "", fmt.Errorf("fail to get api key id: %w", err)
	}

	return record.ID, nil
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}

	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
