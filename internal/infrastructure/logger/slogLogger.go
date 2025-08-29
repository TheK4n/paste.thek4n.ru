// Package logger contains implementations of domain logger.
package logger

import (
	"log/slog"
)

// SlogLogger implementation of domain logger.
type SlogLogger struct {
	slog slog.Logger
}

// NewSlogLogger constructor.
func NewSlogLogger(slg slog.Logger) SlogLogger {
	return SlogLogger{
		slog: slg,
	}
}

// Debug wrapper for slog Debug.
func (l SlogLogger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}

// Info wrapper for slog Info.
func (l SlogLogger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Warn wrapper for slog Warn.
func (l SlogLogger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

// Error wrapper for slog Error.
func (l SlogLogger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}
