package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thek4n/paste.thek4n.ru/internal/config"
	"github.com/thek4n/paste.thek4n.ru/internal/storage"
)

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
	if !checkIsDatabaseAvailable(ctx, app.DB) {
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

func checkIsDatabaseAvailable(ctx context.Context, db storage.KeysDB) bool {
	return db.Ping(ctx)
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
