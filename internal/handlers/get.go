package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

const redirectBody = `<html><head>
<title>303 See Other</title>
</head><body>
<h1>See Other</h1>
<p>The document has moved <a href="%s">here</a>.</p>
</body></html>`

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

	record, err := keys.Get(app.DB, key, 4*time.Second)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := w.Write([]byte("404 Not Found"))
			if writeErr != nil {
				logger.Error(
					"Fail to answer on getting key",
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
			"error", err,
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
			"Redirect to url",
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
		"Got content",
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
		if errors.Is(err, storage.ErrKeyNotFound) {
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
		"Got clicks",
	)
}
