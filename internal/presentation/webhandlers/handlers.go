// Package webhandlers provides handlers
package webhandlers

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/thek4n/paste.thek4n.ru/internal/application/service"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/config"
)

// Handlers struct contains repositories and provides handlers.
type Handlers struct {
	Config             config.CacheValidationConfig
	Version            string
	Logger             slog.Logger
	getService         *service.GetService
	cacheService       *service.CacheService
	HealthcheckEnabled bool
}

// NewHandlers constructor.
func NewHandlers(
	cfg config.CacheValidationConfig,
	version string,
	healthcheckEnabled bool,
	logger slog.Logger,
	getService *service.GetService,
	cacheService *service.CacheService,
) *Handlers {
	return &Handlers{
		Config:             cfg,
		Version:            version,
		HealthcheckEnabled: healthcheckEnabled,
		Logger:             logger,
		getService:         getService,
		cacheService:       cacheService,
	}
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
