package webhandlers

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

type cacheRequestParams struct {
	APIKey       string
	RequestedKey string
	TTL          time.Duration
	Length       int
	Disposable   int
	IsURL        bool
}

type cacheRequestAPIKey struct {
	ID    string
	Valid bool
}

type cacheRequest struct {
	Params           cacheRequestParams
	APIKey           cacheRequestAPIKey
	Body             []byte
	ID               string
	SourceIP         string
	APIKeyAuthorized bool
}

type cacheError struct {
	Message    string
	Err        error
	StatusCode int
}

func (e *cacheError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Cache handle to save key in db.
func (app *Handlers) Cache(w http.ResponseWriter, r *http.Request) {
	req := cacheRequest{}
	req.SourceIP = getClientIP(r)
	req.ID = uuid.NewString()
	var err error

	logger := app.Logger.With("source_ip", req.SourceIP, "request_id", req.ID)
	logger.Debug("Start caching key")

	req.Params, err = app.parseAndValidateRequestParams(r.URL.Query())
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	body, err := readRequestBody(r)
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if req.Params.IsURL && !validateURL(string(body)) {
		handleCacheError(w, fmt.Errorf("invalid url"), logger)
		return
	}

	paramsDisposable := req.Params.Disposable
	if paramsDisposable < 0 || paramsDisposable > math.MaxUint8 {
		handleCacheError(w, fmt.Errorf("disposable counter more then %d or less then 0", math.MaxUint8), logger)
		return
	}

	paramsDisposableChecked := uint8(paramsDisposable)

	paramsLength := req.Params.Length
	if paramsLength < 0 || paramsLength > math.MaxUint8 {
		handleCacheError(w, fmt.Errorf("requested key length more then %d or less then 0", math.MaxUint8), logger)
		return
	}

	paramsLengthChecked := uint8(paramsLength)

	params := objectvalue.CacheRequestParams{
		APIKey:             req.Params.APIKey,
		RequestedKey:       req.Params.RequestedKey,
		SourceIP:           req.SourceIP,
		Body:               body,
		TTL:                req.Params.TTL,
		BodyLen:            int64(len(body)),
		RequestedKeyLength: paramsLengthChecked,
		Disposable:         paramsDisposableChecked,
		IsURL:              req.Params.IsURL,
	}

	recordkey, err := app.cacheService.Serve(params)
	if err != nil {
		handleCacheError(w, err, logger)
		return
	}

	if err := sendSuccessResponse(w, r, string(recordkey)); err != nil {
		handleCacheError(w, err, logger)
		return
	}

	logger.Info("Set key",
		"key", string(recordkey),
		"body_size", len(req.Body),
		"ttl", req.Params.TTL,
		"disposable", req.Params.Disposable,
		"isURL", req.Params.IsURL,
	)
}

func (app *Handlers) parseAndValidateRequestParams(urlQuery url.Values) (cacheRequestParams, error) {
	p := cacheRequestParams{}
	var err error

	p.TTL, err = app.getTTL(urlQuery)
	if err != nil {
		return p, &cacheError{Message: "Invalid 'ttl' parameter", StatusCode: http.StatusUnprocessableEntity}
	}

	p.Length, err = app.getLength(urlQuery)
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

	p.RequestedKey, err = app.getRequestedKey(urlQuery)
	if err != nil {
		return p, &cacheError{Message: err.Error(), StatusCode: http.StatusUnprocessableEntity}
	}

	p.APIKey = urlQuery.Get("apikey")

	return p, nil
}

func readRequestBody(r *http.Request) ([]byte, error) {
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

func sendSuccessResponse(w http.ResponseWriter, r *http.Request, key string) error {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusCreated)

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
	if err == domainerrors.ErrBodyTooLarge {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	if err == domainerrors.ErrAPIKeyNotFound {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err == domainerrors.ErrAPIKeyInvalid {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if err == domainerrors.ErrRequestedKeyExists {
		w.WriteHeader(http.StatusConflict)
		return
	}

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

func (app *Handlers) getTTL(v url.Values) (time.Duration, error) {
	ttlQuery := v.Get("ttl")

	if ttlQuery == "" {
		return app.Config.DefaultTTL(), nil
	}

	ttl, err := time.ParseDuration(ttlQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse duration: %w", err)
	}

	if ttl < app.Config.MinTTL() {
		return 0, fmt.Errorf("TTL can`t be less then %s", app.Config.MinTTL())
	}

	if ttl > app.Config.PrivilegedMaxTTL() {
		return 0, fmt.Errorf("TTL can`t be more then %s", app.Config.PrivilegedMaxTTL())
	}

	return ttl, nil
}

func (app *Handlers) getRequestedKey(v url.Values) (string, error) {
	requestedKey := v.Get("key")

	if requestedKey == "" {
		return "", nil
	}

	if len(requestedKey) > int(app.Config.MaxKeyLength()) {
		return "", fmt.Errorf("requested key length more then max")
	}

	if len(requestedKey) < int(app.Config.PrivilegedMinKeyLength()) {
		return "", fmt.Errorf("requested key length less then min")
	}

	for _, char := range requestedKey {
		if !strings.ContainsRune(app.Config.AllowedKeyChars(), char) {
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

func (app *Handlers) getLength(v url.Values) (int, error) {
	lengthQuery := v.Get("len")

	if lengthQuery == "" {
		return int(app.Config.DefaultKeyLength()), nil
	}

	length, err := strconv.Atoi(lengthQuery)
	if err != nil {
		return 0, fmt.Errorf("fail to parse length: %w", err)
	}

	if length < int(app.Config.PrivilegedMinKeyLength()) {
		return 0, fmt.Errorf("length can`t be less then %d", app.Config.PrivilegedMinKeyLength())
	}

	if length > int(app.Config.MaxKeyLength()) {
		return 0, fmt.Errorf("length can`t be more then %d", app.Config.MaxKeyLength())
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
