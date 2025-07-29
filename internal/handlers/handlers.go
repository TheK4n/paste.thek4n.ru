// Package handlers provides handlers
package handlers

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/thek4n/paste.thek4n.name/internal/apikeys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

// Application struct contains databases connections.
type Application struct {
	Version   string
	DB        storage.KeysDB
	APIKeysDB storage.APIKeysDB
	QuotaDB   storage.QuotaDB
	Broker    apikeys.Broker
	Logger    slog.Logger
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
