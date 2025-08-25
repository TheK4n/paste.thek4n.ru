// Package domainerrors contains errors for domain logic.
package domainerrors

import (
	"errors"
)

// ErrQuotaExhausted error type point that quota exhausted.
var ErrQuotaExhausted = errors.New("quota exhausted")

// ErrBodyTooLarge .
var ErrBodyTooLarge = errors.New("body too large")

// ErrRequestedKeyExists .
var ErrRequestedKeyExists = errors.New("requested key already exists")

// ErrInvalidTTL .
var ErrInvalidTTL = errors.New("invalid ttl")

// ErrInvalidRequestedKeyLength .
var ErrInvalidRequestedKeyLength = errors.New("invalid requested key length")

// ErrInvalidRequestedKey .
var ErrInvalidRequestedKey = errors.New("invalid requested key")

// ErrNonAuthorized .
var ErrNonAuthorized = errors.New("non authorized")

// ErrRecordNotFound .
var ErrRecordNotFound = errors.New("record not found")

// ErrRecordCounterExhausted error type to point that record counter exhausted.
var ErrRecordCounterExhausted = errors.New("record counter exhausted")

// ErrRecordExpired error type to point that record expired.
var ErrRecordExpired = errors.New("record is expired")

// ErrAPIKeyNotFound error type to point that apikey not found.
var ErrAPIKeyNotFound = errors.New("apikey not found")

// ErrQuotaNotFound error type to point that quota not found.
var ErrQuotaNotFound = errors.New("quota not found")
