// Package handlers provides handlers
package handlers

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/thek4n/paste.thek4n.ru/internal/apikeys"
	"github.com/thek4n/paste.thek4n.ru/internal/storage"
)

// Application struct contains databases connections.
type Application struct {
	Version            string
	DB                 storage.KeysDB
	APIKeysDB          storage.APIKeysDB
	QuotaDB            storage.QuotaDB
	Broker             apikeys.Broker
	Logger             slog.Logger
	HealthcheckEnabled bool
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
