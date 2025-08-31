// Package objectvalue contains domain object values.
package objectvalue

import (
	"time"

	"github.com/google/uuid"
)

// CacheRequestID type for request id.
type CacheRequestID uuid.UUID

// ExpirationDate timer to expirate record.
type ExpirationDate struct {
	date    time.Time
	eternal bool
}

// NewExpirationDateFromTTL constructor.
func NewExpirationDateFromTTL(t time.Duration) ExpirationDate {
	eternal := t == 0
	return ExpirationDate{
		date:    time.Now().Add(t),
		eternal: eternal,
	}
}

// Expired returns is date after then now.
func (e ExpirationDate) Expired() bool {
	if e.eternal {
		return false
	}
	return time.Now().After(e.date)
}

// Eternal returns is expiration date eternal.
func (e ExpirationDate) Eternal() bool {
	return e.eternal
}

// Until returns duration until e.
func (e ExpirationDate) Until() time.Duration {
	if e.eternal {
		return 0
	}
	return time.Until(e.date)
}

// DisposableCounter counter for record.
type DisposableCounter struct {
	value   uint8
	eternal bool
}

// NewDisposableCounter validate value and returns valid DisposableCounter.
func NewDisposableCounter(value uint8, eternal bool) DisposableCounter {
	c := DisposableCounter{
		value:   value,
		eternal: eternal,
	}
	return c
}

// Value getter.
func (d DisposableCounter) Value() uint8 {
	return d.value
}

// Sub decreases counter.
func (d DisposableCounter) Sub() DisposableCounter {
	if d.eternal {
		return d
	}

	if d.value == 0 {
		return d
	}

	d.value--
	return d
}

// Eternal returns is DisposableCounter eternal.
func (d DisposableCounter) Eternal() bool {
	return d.eternal
}

// Exhausted returns true if counter less then 1.
func (d DisposableCounter) Exhausted() bool {
	if d.eternal {
		return false
	}
	return d.value < 1
}

// ClicksCounter record clicks counter.
type ClicksCounter struct {
	value uint32
}

// NewClicksCounter returns valid ClicksCounter.
func NewClicksCounter(value uint32) ClicksCounter {
	cc := ClicksCounter{value: value}
	return cc
}

// Value getter.
func (c ClicksCounter) Value() uint32 {
	return c.value
}

// Increase adds 1 to record clicks counter.
func (c ClicksCounter) Increase() ClicksCounter {
	c.value++
	return c
}

// CacheRequestParams represents cache request params.
type CacheRequestParams struct {
	APIKey             string
	RequestedKey       string
	SourceIP           string
	Body               []byte
	TTL                time.Duration
	BodyLen            int64
	RequestedKeyLength uint8
	Disposable         uint8
	IsURL              bool
}
