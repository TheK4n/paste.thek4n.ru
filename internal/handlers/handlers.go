package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

const ONE_MEBIBYTE = 1048576
const UNPREVELEGED_MAX_BODY_SIZE = ONE_MEBIBYTE
const PREVELEGED_MAX_BODY_SIZE = ONE_MEBIBYTE * 100

const SECONDS_IN_MONTH = 60 * 60 * 24 * 30
const DEFAULT_TTL_SECONDS = time.Second * SECONDS_IN_MONTH

const MIN_TTL = time.Second * 0

const SECONDS_IN_YEAR = 60 * 60 * 24 * 30 * 12
const MAX_TTL = time.Second * SECONDS_IN_YEAR

const MAX_KEY_LENGTH = 20
const DEFAULT_KEY_LENGTH = 14
const UNPRIVELEGED_MIN_KEY_LENGTH = 14
const PRIVELEGED_MIN_KEY_LENGTH = 3

const HEALTHCHECK_TIMEOUT = time.Second * 3

const REDIRECT_BODY = `<html><head>
<title>303 See Other</title>
</head><body>
<h1>See Other</h1>
<p>The document has moved <a href="%s">here</a>.</p>
</body></html>`

type Application struct {
	Version   string
	DB        storage.KeysDB
	ApiKeysDB storage.APIKeysDB
	QuotaDB   storage.QuotaDB
}

type HealthcheckResponse struct {
	Version      string `json:"version"`
	Availability bool   `json:"availability"`
	Msg          string `json:"msg"`
}

// Checks database availability and returns version
func (app *Application) Healthcheck(w http.ResponseWriter, r *http.Request) {
	availability := true
	msg := "ok"

	ctx, cancel := context.WithTimeout(context.Background(), HEALTHCHECK_TIMEOUT)
	defer cancel()

	if !app.DB.Ping(ctx) {
		availability = false
		msg = "Error connection to database"
	}

	resp := &HealthcheckResponse{
		Version:      app.Version,
		Availability: availability,
		Msg:          msg,
	}

	answer, err := json.Marshal(resp)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte(answer))

	if err != nil {
		log.Printf("Error on answer healthcheck: %s, suffered user %s", err.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (app *Application) Cache(w http.ResponseWriter, r *http.Request) {
	authorized := false
	apikey := r.URL.Query().Get("apikey")
	if apikey != "" {
		authorized = app.validateApikey(apikey)
	}

	if !authorized {
		ip := getClientIP(r)
		quotaValid, err := app.QuotaDB.IsQuotaValid(context.Background(), ip)
		if err != nil {
			log.Printf(
				"Error on checking quota: %s. Response to client %s with code %d",
				err.Error(),
				r.RemoteAddr,
				http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !quotaValid {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "Your quota for today is exhausted.")
			return
		}

		err = app.QuotaDB.CreateAndSubOrJustSub(context.Background(), ip)
		if err != nil {
			log.Printf(
				"Error on sub quota: %s. Response to client %s with code %d",
				err.Error(),
				r.RemoteAddr,
				http.StatusInternalServerError,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	}

	ttl, errGetTTL := getTTL(r)
	if errGetTTL != nil {
		log.Printf(
			"Error on parsing ttl: %s. Response to client %s with code %d",
			errGetTTL.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, "Invalid 'ttl' parameter")
		return
	}

	if ttl == time.Duration(0) {
		if !authorized {
			log.Printf(
				"Unathorized attempt to set persist key",
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	length, errGetLength := getLength(r)
	if errGetLength != nil {
		log.Printf(
			"Error on parsing length: %s. Response to client %s with code %d",
			errGetLength.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, "Invalid 'len' parameter")
		return
	}

	if length < UNPRIVELEGED_MIN_KEY_LENGTH {
		if !authorized {
			log.Printf(
				"Unathorized attempt to set short key with length %d",
				length,
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	disposable, errGetDisposable := getDisposable(r)
	if errGetDisposable != nil {
		log.Printf(
			"Error on validating disposable argument: %s. Response to client %s with code %d",
			errGetDisposable.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, "Invalid 'disposable' parameter")
		return
	}

	isUrl, errGetURL := getURL(r)
	if errGetURL != nil {
		log.Printf(
			"Error on validating url argument: %s. Response to client %s with code %d",
			errGetURL.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, "Invalid 'url' parameter")
		return
	}

	requestedKey, errGetRequestedKey := getRequestedKey(r)
	if errGetRequestedKey != nil {
		log.Printf(
			"Error on validating 'key' argument: %s. Response to client %s with code %d",
			errGetRequestedKey.Error(),
			r.RemoteAddr,
			http.StatusUnprocessableEntity,
		)
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, errGetRequestedKey.Error())
		return
	}

	if !authorized {
		if requestedKey != "" {
			log.Printf(
				"Unathorized attempt to set custom key",
			)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	if !authorized {
		if r.ContentLength > UNPREVELEGED_MAX_BODY_SIZE {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			fmt.Fprintf(w, "Body too large. Maximum is %d bytes", UNPREVELEGED_MAX_BODY_SIZE)
			return
		}
	}

	if r.ContentLength > PREVELEGED_MAX_BODY_SIZE {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		fmt.Fprintf(w, "Body too large. Maximum is %d bytes", PREVELEGED_MAX_BODY_SIZE)
		return
	}

	body, readBodyErr := io.ReadAll(r.Body)

	if readBodyErr != io.EOF && readBodyErr != nil {
		log.Printf(
			"Error on reading body: %s. Response to client %s with code %d",
			readBodyErr.Error(),
			r.RemoteAddr,
			http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if isUrl {
		body = []byte(strings.TrimSpace(string(body)))
		if !validateUrl(string(body)) {
			log.Printf(
				"Error on validating url body. Response to client %s with code %d",
				r.RemoteAddr,
				http.StatusUnprocessableEntity,
			)
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, "Invalid 'url' parameter")
			return
		}
	}

	var record storage.KeyRecord
	record.Body = body
	record.Disposable = disposable != 0
	record.Countdown = disposable
	record.URL = isUrl
	record.Clicks = 0

	key, err := keys.Cache(app.DB, 4*time.Second, requestedKey, ttl, length, record)
	if err != nil {
		if err == keys.ErrKeyAlreadyTaken || errors.Unwrap(err) == keys.ErrKeyAlreadyTaken {
			log.Printf("Try to take already taken key from %s: Error: %s", r.RemoteAddr, err)
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, "Key already taken")
			return
		}

		log.Printf("Error on setting key: %s, suffered user %s", err, r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	scheme := detectScheme(r)

	_, answerErr := fmt.Fprintf(w, "%s%s/%s/", scheme, r.Host, key)
	if answerErr != nil {
		log.Printf("Error on answer: %s", answerErr.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("Set key '%s' size=%d ttl=%s countdown=%d url=%t", key, len(body), ttl, disposable, isUrl)
}

func (app *Application) Get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	record, getKeyErr := keys.Get(app.DB, key, 4*time.Second)

	if getKeyErr != nil {
		if getKeyErr == storage.ErrKeyNotFound || errors.Unwrap(getKeyErr) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := w.Write([]byte("404 Not Found"))
			if writeErr != nil {
				log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Not found by key '%s' from %s", key, r.RemoteAddr)
			return
		} else {
			log.Printf(
				"Error on getting key: %s, suffered user %s",
				getKeyErr.Error(),
				r.RemoteAddr,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if record.URL {
		answer := make([]byte, 0)
		answer = fmt.Appendf(answer, REDIRECT_BODY, string(record.Body))
		w.Header().Set(http.CanonicalHeaderKey("content-type"), http.DetectContentType(answer))
		http.Redirect(w, r, strings.TrimSpace(string(record.Body)), http.StatusSeeOther)
		_, writeErr := w.Write(answer)
		if writeErr != nil {
			log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Printf("Redirect url by key '%s' from %s", key, r.RemoteAddr)
		return
	}

	w.Header().Set(http.CanonicalHeaderKey("content-type"), http.DetectContentType(record.Body))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(record.Body)
	if writeErr != nil {
		log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Get content by key '%s' from %s", key, r.RemoteAddr)
}

func (app *Application) GetClicks(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	clicks, err := keys.GetClicks(app.DB, key, 4*time.Second)
	if err != nil {
		if err == storage.ErrKeyNotFound || errors.Unwrap(err) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := fmt.Fprint(w, "404 Not Found")
			if writeErr != nil {
				log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Not found by key '%s' from %s", key, r.RemoteAddr)
			return
		} else {
			log.Printf(
				"Error on getting key: %s, suffered user %s",
				err.Error(),
				r.RemoteAddr,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	body := []byte(strconv.Itoa(clicks))
	w.Header().Set(http.CanonicalHeaderKey("content-type"), http.DetectContentType(body))
	w.WriteHeader(http.StatusOK)

	_, writeErr := w.Write(body)
	if writeErr != nil {
		log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Get clicks by key '%s' from %s", key, r.RemoteAddr)
}

func getTTL(r *http.Request) (time.Duration, error) {
	ttlQuery := r.URL.Query().Get("ttl")

	if ttlQuery == "" {
		return DEFAULT_TTL_SECONDS, nil
	}

	ttl, err := time.ParseDuration(ttlQuery)
	if err != nil {
		return 0, err
	}

	if ttl < MIN_TTL {
		return 0, fmt.Errorf("TTL can`t be less then %s", MIN_TTL)
	}

	if ttl > MAX_TTL {
		return 0, fmt.Errorf("TTL can`t be more then %s", MAX_TTL)
	}

	return ttl, nil
}

func getRequestedKey(r *http.Request) (string, error) {
	requestedKey := r.URL.Query().Get("key")

	if requestedKey == "" {
		return "", nil
	}

	if len(requestedKey) > MAX_KEY_LENGTH {
		return "", fmt.Errorf("%s", "Requested key length more then max")
	}

	if len(requestedKey) < PRIVELEGED_MIN_KEY_LENGTH {
		return "", fmt.Errorf("%s", "Requested key length less then min")
	}

	for _, char := range requestedKey {
		if !strings.ContainsRune(keys.CHARSET, char) {
			return "", fmt.Errorf("%s", "Requested key contains illegal char")
		}
	}

	return requestedKey, nil
}

func getDisposable(r *http.Request) (int, error) {
	disposableQuery := r.URL.Query().Get("disposable")

	if disposableQuery == "" {
		return 0, nil
	}

	disposable, err := strconv.Atoi(disposableQuery)
	if err != nil {
		return 0, err
	}

	if disposable < 0 {
		return 0, fmt.Errorf("Disposable argument can`t be less then zero")
	}

	if disposable > 255 {
		return 0, fmt.Errorf("Disposable argument can`t be more then 255")
	}

	return disposable, nil
}

func getLength(r *http.Request) (int, error) {
	lengthQuery := r.URL.Query().Get("len")

	if lengthQuery == "" {
		return DEFAULT_KEY_LENGTH, nil
	}

	length, err := strconv.Atoi(lengthQuery)
	if err != nil {
		return 0, err
	}

	if length < PRIVELEGED_MIN_KEY_LENGTH {
		return 0, fmt.Errorf("Length can`t be less then %d", PRIVELEGED_MIN_KEY_LENGTH)
	}

	if length > MAX_KEY_LENGTH {
		return 0, fmt.Errorf("Length can`t be more then %d", MAX_KEY_LENGTH)
	}

	return length, nil
}

func getURL(r *http.Request) (bool, error) {
	URLQuery := r.URL.Query().Get("url")

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

func detectScheme(r *http.Request) string {
	if r.TLS == nil {
		return "http://"
	} else {
		return "https://"
	}
}

func validateUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func (app *Application) validateApikey(str string) bool {
	record, err := app.ApiKeysDB.Get(context.Background(), str)
	if err != nil {
		return false
	}

	return record.Valid
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
