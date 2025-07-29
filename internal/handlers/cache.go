package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thek4n/paste.thek4n.name/internal/config"
	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
	apikeysm "github.com/thek4n/paste.thek4n.name/pkg/apikeys"
)

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
		logger = logger.With("authorized", true, "apikey_id", cacheReq.APIKeyID)
		logger.Debug("Authorize apikey")
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

	if !cacheReq.Authorized {
		if err := app.checkQuota(remoteAddr, logger); err != nil {
			handleCacheError(w, err, logger)
			return
		}
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
		apikeyRecord, err := app.getAPIKeyRecord(req.APIKey)
		if errors.Is(err, storage.ErrKeyNotFound) {
			logger.Debug("Detect usage not existing apikey", "answer_code", http.StatusUnauthorized)
			return nil, &cacheError{Message: "Cannot authorize provided apikey", StatusCode: http.StatusUnauthorized}
		}
		if err != nil {
			logger.Error("Fail to check apikey", "error", err, "answer_code", http.StatusInternalServerError)
			return nil, &cacheError{Message: "Failed to check apikey", StatusCode: http.StatusInternalServerError, Err: err}
		}

		req.Authorized = apikeyRecord.Valid
		req.APIKeyID = apikeyRecord.ID

		if !req.Authorized {
			logger.Warn("Detect usage revoked apikey", "answer_code", http.StatusUnauthorized, "apikey_id", req.APIKeyID)
			return nil, &cacheError{Message: "Cannot authorize provided apikey", StatusCode: http.StatusUnauthorized}
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
		logger.Error(cacheErr.Message,
			"error", cacheErr.Err,
			"answer_code", http.StatusInternalServerError,
		)
	}

	w.WriteHeader(cacheErr.StatusCode)
	if cacheErr.Message != "" {
		if _, err := fmt.Fprint(w, cacheErr.Message); err != nil {
			logger.Error("Error on answer error",
				"error", cacheErr.Err,
				"answer_code", http.StatusInternalServerError,
			)
		}
	}
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

func (app *Application) getAPIKeyRecord(str string) (storage.APIKeyRecord, error) {
	record, err := app.APIKeysDB.Get(context.Background(), str)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return record, fmt.Errorf("apikey not found: %w", err)
	}
	if err != nil {
		return record, fmt.Errorf("fail to get api key: %w", err)
	}

	return record, nil
}
