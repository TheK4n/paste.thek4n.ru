// Package config is configuration for paste service.
package config

import (
	"time"
)

// Body size.
const (
	oneMebibyte             = 1048576
	UnprevelegedMaxBodySize = oneMebibyte
	PrevelegedMaxBodySize   = 100 * oneMebibyte
)

// TTL.
const (
	secondsInMonth = 60 * 60 * 24 * 30
	DefaultTTL     = time.Second * secondsInMonth
	MinTTL         = time.Second * 0
	secondsInYear  = secondsInMonth * 12
	MaxTTL         = time.Second * secondsInYear
)

// Key length.
const (
	MaxKeyLength             = 20
	DefaultKeyLength         = 14
	UnprivelegedMinKeyLength = 14
	PrivelegedMinKeyLength   = 3
)

// Healthcheck.
const (
	HealthcheckTimeout = time.Second * 3
)

// Key generation config.
const (
	AttemptsToIncreaseKeyMinLenght = 20
	Charset                        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// Quota config.
const (
	QuotaResetPeriod = 24 * time.Hour
	Quota            = 50
)

// Broker.
const (
	APIKeyUsageExchange = "apikeysusage"
)

// Compress.
const (
	CompressThresholdBytes = 4096
)
