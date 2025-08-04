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

	"github.com/thek4n/paste.thek4n.ru/internal/config"
	"github.com/thek4n/paste.thek4n.ru/internal/keys"
	"github.com/thek4n/paste.thek4n.ru/internal/storage"
	apikeysm "github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
)

type cacheRequestParams struct {
	APIKey       string
	TTL          time.Duration
	Length       int
	Disposable   int
	IsURL        bool
	RequestedKey string
}

type cacheRequestAPIKey struct {
	ID    string
	Valid bool
}

type cacheRequest struct {
	ID               string
	SourceIP         string
	Body             []byte
	APIKeyAuthorized bool
	Params           cacheRequestParams
	APIKey           cacheRequestAPIKey
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
	req := cacheRequest{APIKeyAuthorized: false}
	req.SourceIP = getClientIP(r)
	req.ID = uuid.NewString()
	var err error

	logger := app.Logger.With("source_ip", req.SourceIP, "request_id", req.ID)
	logger.Debug("Start caching key")

	req.Params, err = parseAndValidateRequestParams(r.URL.Query())
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if req.Params.APIKey != "" {
		req.APIKeyAuthorized, err = authorizeAPIKey(app.APIKeysDB, req.Params.APIKey, logger)
		if err != nil {
			handleCacheError(w, err, logger)
			return
		}
	}

	if req.APIKeyAuthorized {
		req.APIKey.ID, err = getAPIKeyID(app.APIKeysDB, req.Params.APIKey)
		if err != nil {
			handleCacheError(w, err, logger)
			return
		}
		logger = logger.With("authorized", true, "apikey_id", req.APIKey.ID)
		logger.Debug("Authorize apikey")
	}

	maxBodySize := config.UnprevelegedMaxBodySize
	if req.APIKeyAuthorized {
		maxBodySize = config.PrevelegedMaxBodySize
	}

	req.Body, err = readRequestBody(r, maxBodySize)
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if err := validateCacheRequest(&req); err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if req.APIKeyAuthorized {
		app.logAPIKeyUsage(&req, logger)
	}

	key, err := saveKey(app.DB, &req)
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if !req.APIKeyAuthorized {
		if err := processQuota(app.QuotaDB, req.SourceIP, logger); err != nil {
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
		"body_size", len(req.Body),
		"ttl", req.Params.TTL,
		"disposable", req.Params.Disposable,
		"isURL", req.Params.IsURL,
	)
}

func parseAndValidateRequestParams(urlQuery url.Values) (cacheRequestParams, error) {
	p := cacheRequestParams{}
	var err error

	p.TTL, err = getTTL(urlQuery)
	if err != nil {
		return p, &cacheError{Message: "Invalid 'ttl' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	p.Length, err = getLength(urlQuery)
	if err != nil {
		return p, &cacheError{Message: "Invalid 'len' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	p.Disposable, err = getDisposable(urlQuery)
	if err != nil {
		return p, &cacheError{Message: "Invalid 'disposable' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	p.IsURL, err = getURL(urlQuery)
	if err != nil {
		return p, &cacheError{Message: "Invalid 'url' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	p.RequestedKey, err = getRequestedKey(urlQuery)
	if err != nil {
		return p, &cacheError{Message: err.Error(), StatusCode: http.StatusUnprocessableEntity}
	}

	p.APIKey = urlQuery.Get("apikey")

	return p, nil
}

func authorizeAPIKey(db storage.APIKeysDB, apikey string, logger *slog.Logger) (bool, error) {
	apikeyRecord, err := getAPIKeyRecord(db, apikey)
	if errors.Is(err, storage.ErrKeyNotFound) {
		logger.Debug("Detect usage not existing apikey", "answer_code", http.StatusUnauthorized)
		return false, &cacheError{Message: "Cannot authorize provided apikey", StatusCode: http.StatusUnauthorized}
	}
	if err != nil {
		logger.Error("Fail to check apikey", "error", err, "answer_code", http.StatusInternalServerError)
		return false, &cacheError{Message: "Failed to check apikey", StatusCode: http.StatusInternalServerError, Err: err}
	}

	if !apikeyRecord.Valid {
		logger.Warn("Detect usage revoked apikey", "answer_code", http.StatusUnauthorized, "apikey_id", apikeyRecord.ID)
		return false, &cacheError{Message: "Cannot authorize provided apikey", StatusCode: http.StatusUnauthorized}
	}

	return true, nil
}

func processQuota(db storage.QuotaDB, remoteAddr string, logger *slog.Logger) error {
	quotaValid, err := db.IsQuotaValid(context.Background(), remoteAddr)
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

	if err := db.ReduceQuota(context.Background(), remoteAddr, logger); err != nil {
		return &cacheError{
			Message:    "Failed to reduce quota",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}

	return nil
}

func readRequestBody(r *http.Request, maxBodySize int64) ([]byte, error) {
	if r.ContentLength > maxBodySize {
		return nil, &cacheError{
			Message:    fmt.Sprintf("Body too large. Maximum is %d bytes", maxBodySize),
			StatusCode: http.StatusRequestEntityTooLarge,
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil && err != io.EOF {
		return nil, &cacheError{
			Message:    "Failed to read body",
			StatusCode: http.StatusInternalServerError,
			Err:        err,
		}
	}

	return body, nil
}

func validateCacheRequest(req *cacheRequest) error {
	if req.Params.TTL == 0 && !req.APIKeyAuthorized {
		return &cacheError{
			Message:    "Unauthorized attempt to set persist key",
			StatusCode: http.StatusUnauthorized,
		}
	}

	if req.Params.Length < config.UnprivelegedMinKeyLength && !req.APIKeyAuthorized {
		return &cacheError{
			Message:    "Unauthorized attempt to set short key",
			StatusCode: http.StatusUnauthorized,
		}
	}

	if req.Params.RequestedKey != "" && !req.APIKeyAuthorized {
		return &cacheError{
			Message:    "Unauthorized attempt to set custom key",
			StatusCode: http.StatusUnauthorized,
		}
	}

	if req.Params.IsURL {
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
func (app *Application) logAPIKeyUsage(req *cacheRequest, logger *slog.Logger) {
	var reason apikeysm.UsageReason
	switch {
	case req.Params.TTL == 0:
		reason = apikeysm.UsageReason_PERSISTKEY
	case req.Params.Length < config.UnprivelegedMinKeyLength:
		reason = apikeysm.UsageReason_CUSTOMKEYLEN
	case req.Params.RequestedKey != "":
		reason = apikeysm.UsageReason_CUSTOMKEY
	case len(req.Body) > int(config.UnprevelegedMaxBodySize):
		reason = apikeysm.UsageReason_LARGEBODY
	default:
		return
	}

	if err := app.Broker.SendAPIKeyUsageLog(req.APIKey.ID, reason, req.SourceIP); err != nil {
		logger.Warn("Fail to publish to broker", "error", err)
	}
	logger.Debug("Sent apikey usage reason to broker", "reason", reason)
}

func saveKey(db storage.KeysDB, req *cacheRequest) (string, error) {
	record := storage.KeyRecord{
		Body:       req.Body,
		Disposable: req.Params.Disposable != 0,
		Countdown:  req.Params.Disposable,
		URL:        req.Params.IsURL,
		Clicks:     0,
	}

	var key string
	var err error

	if req.Params.RequestedKey == "" {
		key, err = keys.CacheGeneratedKey(db, 4*time.Second, req.Params.TTL, req.Params.Length, record)
	} else {
		key, err = keys.CacheRequestedKey(db, 4*time.Second, req.Params.RequestedKey, req.Params.TTL, record)
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

func validateURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func getAPIKeyRecord(db storage.APIKeysDB, key string) (storage.APIKeyRecord, error) {
	record, err := db.Get(context.Background(), key)
	if errors.Is(err, storage.ErrKeyNotFound) {
		return record, fmt.Errorf("apikey not found: %w", err)
	}
	if err != nil {
		return record, fmt.Errorf("fail to get api key: %w", err)
	}

	return record, nil
}

func getAPIKeyID(db storage.APIKeysDB, key string) (string, error) {
	record, err := getAPIKeyRecord(db, key)
	if err != nil {
		return "", fmt.Errorf("fail to get apikey record: %w", err)
	}

	return record.ID, nil
}
