// Package config is configuration for paste service.
package config

import (
	"time"
)

// CacheValidationConfig contains getters for validation values.
type CacheValidationConfig interface {
	UnprivilegedMaxBodySize() int64
	PrivilegedMaxBodySize() int64

	MinTTL() time.Duration
	DefaultTTL() time.Duration
	UnprivilegedMaxTTL() time.Duration
	PrivilegedMaxTTL() time.Duration

	MaxKeyLength() uint8
	DefaultKeyLength() uint8
	UnprivilegedMinKeyLength() uint8
	PrivilegedMinKeyLength() uint8

	AllowedKeyChars() string
}

// QuotaConfig contains getters for quota config values.
type QuotaConfig interface {
	QuotaResetPeriod() time.Duration
	Quota() uint32
}

// CachingConfig contains getters for caching config values.
type CachingConfig interface {
	CompressThresholdBytes() uint16
	MaxBodySize() int64
	AttemptsToIncreaseKeyMinLength() uint8
	KeysCharset() string
}

// DefaultCacheValidationConfig contains default values for cache validataion.
type DefaultCacheValidationConfig struct{}

// UnprivilegedMaxBodySize max body size for unprivileged.
func (c DefaultCacheValidationConfig) UnprivilegedMaxBodySize() int64 {
	return oneMebibyte
}

// PrivilegedMaxBodySize max body size for privileged.
func (c DefaultCacheValidationConfig) PrivilegedMaxBodySize() int64 {
	return 100 * oneMebibyte
}

// MinTTL min ttl.
func (c DefaultCacheValidationConfig) MinTTL() time.Duration {
	return time.Second * 1
}

// DefaultTTL default ttl.
func (c DefaultCacheValidationConfig) DefaultTTL() time.Duration {
	return time.Hour * hoursInMonth
}

// UnprivilegedMaxTTL max ttl for unprivileged.
func (c DefaultCacheValidationConfig) UnprivilegedMaxTTL() time.Duration {
	return time.Hour * hoursInMonth
}

// PrivilegedMaxTTL max ttl for privileged.
func (c DefaultCacheValidationConfig) PrivilegedMaxTTL() time.Duration {
	return time.Hour * hoursInMonth * monthsInYear
}

// MaxKeyLength max key length.
func (c DefaultCacheValidationConfig) MaxKeyLength() uint8 {
	return 20
}

// DefaultKeyLength default key length.
func (c DefaultCacheValidationConfig) DefaultKeyLength() uint8 {
	return 14
}

// UnprivilegedMinKeyLength min key length for unprivileged.
func (c DefaultCacheValidationConfig) UnprivilegedMinKeyLength() uint8 {
	return 8
}

// PrivilegedMinKeyLength min key length for privileged.
func (c DefaultCacheValidationConfig) PrivilegedMinKeyLength() uint8 {
	return 3
}

// AllowedKeyChars allowed charset for key generation.
func (c DefaultCacheValidationConfig) AllowedKeyChars() string {
	return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
}

// DefaultQuotaConfig contains getters for defaults quota config.
type DefaultQuotaConfig struct{}

// QuotaResetPeriod quota reset period.
func (c DefaultQuotaConfig) QuotaResetPeriod() time.Duration {
	return hoursInDay * time.Hour
}

// Quota default quota for quota reset period.
func (c DefaultQuotaConfig) Quota() uint32 {
	return 50
}

// DefaultCachingConfig contains getters for defaults caching config.
type DefaultCachingConfig struct{}

// CompressThresholdBytes when body need to compress.
func (c DefaultCachingConfig) CompressThresholdBytes() uint16 {
	return 4096
}

// MaxBodySize max body size.
func (c DefaultCachingConfig) MaxBodySize() int64 {
	return 100 * oneMebibyte
}

// AttemptsToIncreaseKeyMinLength number of attempts when key length is grow.
func (c DefaultCachingConfig) AttemptsToIncreaseKeyMinLength() uint8 {
	return 20
}

// KeysCharset default key generation charset.
func (c DefaultCachingConfig) KeysCharset() string {
	return "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
}

// Body size.
const (
	oneMebibyte int64 = 1048576
)

// TTL.
const (
	hoursInDay   = 24
	daysInMonth  = 30
	hoursInMonth = hoursInDay * daysInMonth
	monthsInYear = 12
)
