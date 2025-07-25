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
	Authorized bool
}

func sendTextResponse(w http.ResponseWriter, message string, statusCode int) error {
	w.WriteHeader(statusCode)
	_, err := fmt.Fprintf(w, "%s", message)
	return fmt.Errorf("fail to response: %w", err)
}

// Cache handle request to set key.
func (app *Application) Cache(w http.ResponseWriter, r *http.Request) {
	remoteAddr := getClientIP(r)
	requestUUID := uuid.NewString()
	urlValues := r.URL.Query()
	req := cacheRequest{}
	req.Authorized = false

	logger := app.Logger.With(
		"source_ip", remoteAddr,
		"request_id", requestUUID,
	)
	logger.Debug("Start caching key")

	apikey := urlValues.Get("apikey")
	if apikey != "" {
		var err error
		req.Authorized, err = app.validateApikey(apikey)
		if err != nil {
			logger.Warn("Fail to check apikey",
				"error", err,
				"answer_code", http.StatusInternalServerError,
			)
			if err := sendTextResponse(w, "", http.StatusInternalServerError); err != nil {
				logger.Error("Fail to answer on cache request",
					"error", err,
					"answer_code", http.StatusInternalServerError,
				)
			}
			return
		}
	}

	apikeyID := ""
	var getAPIKeyIDErr error
	if req.Authorized {
		apikeyID, getAPIKeyIDErr = app.getAPIKeyID(apikey)
		if getAPIKeyIDErr != nil {
			logger.Warn("fail to fetch apikey id")
		}
	}

	if !req.Authorized {
		quotaValid, err := app.QuotaDB.IsQuotaValid(context.Background(), remoteAddr)
		if err != nil {
			logger.Error(
				"Fail to check quota",
				"error", err,
				"answer_code", http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !quotaValid {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, "Your quota for today is exhausted.")
			return
		}

		err = app.QuotaDB.ReduceQuota(context.Background(), remoteAddr, logger)
		if err != nil {
			logger.Error(
				"Fail to reduce quota",
				"error", err,
				"answer_code", http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if req.Authorized {
		logger.Info(
			"Authorize apikey",
			"apikey_id", apikeyID,
		)
	}

	ttl, errGetTTL := getTTL(urlValues)
	if errGetTTL != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'ttl' parameter")
		return
	}

	if ttl == time.Duration(0) {
		if !req.Authorized {
			logger.Warn(
				"Unathorized attempt to set persist key",
				"answer_code", http.StatusUnauthorized,
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		err := app.Broker.SendAPIKeyUsageLog(apikeyID, apikeysm.UsageReason_PERSISTKEY, remoteAddr)
		if err != nil {
			logger.Warn(
				"fail to publish to broker",
				"error", err,
			)
		}
	}

	length, errGetLength := getLength(urlValues)
	if errGetLength != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'len' parameter")
		return
	}

	if length < config.UnprivelegedMinKeyLength {
		if !req.Authorized {
			logger.Warn(
				"Unathorized attempt to set short key",
				"requested_key_length", length,
				"answer_code", http.StatusUnauthorized,
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		err := app.Broker.SendAPIKeyUsageLog(apikeyID, apikeysm.UsageReason_CUSTOMKEYLEN, remoteAddr)
		if err != nil {
			logger.Warn(
				"fail to publish to broker",
				"error", err,
			)
		}
	}

	disposable, errGetDisposable := getDisposable(urlValues)
	if errGetDisposable != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'disposable' parameter")
		return
	}

	isURL, errGetURL := getURL(urlValues)
	if errGetURL != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, "Invalid 'url' parameter")
		return
	}

	requestedKey, errGetRequestedKey := getRequestedKey(urlValues)
	if errGetRequestedKey != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, errGetRequestedKey.Error())
		return
	}

	if !req.Authorized {
		if requestedKey != "" {
			logger.Warn(
				"Unathorized attempt to set custom key",
				"requested_key", requestedKey,
				"answer_code", http.StatusUnauthorized,
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if req.Authorized {
		if requestedKey != "" {
			err := app.Broker.SendAPIKeyUsageLog(apikeyID, apikeysm.UsageReason_CUSTOMKEY, remoteAddr)
			if err != nil {
				logger.Warn(
					"fail to publish to broker",
					"error", err,
				)
			}
		}
	}

	if !req.Authorized {
		if r.ContentLength > config.UnprevelegedMaxBodySize {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = fmt.Fprintf(w, "Body too large. Maximum is %d bytes", config.UnprevelegedMaxBodySize)
			return
		}
	}
	if req.Authorized {
		if r.ContentLength > config.UnprevelegedMaxBodySize {
			err := app.Broker.SendAPIKeyUsageLog(apikeyID, apikeysm.UsageReason_LARGEBODY, remoteAddr)
			if err != nil {
				logger.Warn(
					"fail to publish to broker",
					"error", err,
				)
			}
		}
	}

	if r.ContentLength > config.PrevelegedMaxBodySize {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = fmt.Fprintf(w, "Body too large. Maximum is %d bytes", config.PrevelegedMaxBodySize)
		return
	}

	body, readBodyErr := io.ReadAll(r.Body)

	if readBodyErr != io.EOF && readBodyErr != nil {
		logger.Error(
			"Fail to read body",
			"error", readBodyErr,
			"answer_code", http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isURL {
		body = []byte(strings.TrimSpace(string(body)))
		if !validateURL(string(body)) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = fmt.Fprint(w, "Invalid 'url' parameter")
			return
		}
	}

	var record storage.KeyRecord
	record.Body = body
	record.Disposable = disposable != 0
	record.Countdown = disposable
	record.URL = isURL
	record.Clicks = 0

	var key string
	var err error

	if requestedKey == "" {
		key, err = keys.CacheGeneratedKey(app.DB, 4*time.Second, ttl, length, record)
	} else {
		key, err = keys.CacheRequestedKey(app.DB, 4*time.Second, requestedKey, ttl, record)
		if err != nil {
			if errors.Is(err, keys.ErrKeyAlreadyTaken) {
				logger.Warn(
					"Attempt to take already taken key",
					"answer_code", http.StatusConflict,
				)
				w.WriteHeader(http.StatusConflict)
				_, _ = fmt.Fprint(w, "Key already taken")
				return
			}
		}
	}

	if err != nil {
		logger.Error(
			"Fail to set key",
			"error", err,
			"answer_code", http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	proto := detectProto(r)

	_, answerErr := fmt.Fprintf(w, "%s://%s/%s/", proto, r.Host, key)
	if answerErr != nil {
		logger.Error(
			"Fail to answer",
			"error", answerErr,
			"answer_code", http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Info(
		"Set key",
		"key", key,
		"body_size", len(body),
		"ttl", ttl,
		"disposable", disposable,
		"isURL", isURL,
	)
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
