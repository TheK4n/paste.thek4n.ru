// Package objectvalue contains domain object values.
package objectvalue

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// CacheRequestID type for request id.
type CacheRequestID uuid.UUID

// ExpirationDate timer to expirate record.
type ExpirationDate struct {
	time.Time
	eternal bool
}

// NewExpirationDateFromTTL constructor.
func NewExpirationDateFromTTL(t time.Duration) ExpirationDate {
	eternal := t == 0
	return ExpirationDate{
		Time:    time.Now().Add(t),
		eternal: eternal,
	}
}

// Expired returns is date after then now.
func (e ExpirationDate) Expired() bool {
	if e.eternal {
		return false
	}
	return time.Now().After(e.Time)
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
	return time.Until(e.Time)
}

// DisposableCounter counter for record.
type DisposableCounter struct {
	atomic.Int32
	eternal bool
}

// NewDisposableCounter validate value and returns valid DisposableCounter.
func NewDisposableCounter(value int32, eternal bool) (*DisposableCounter, error) {
	if value > 255 {
		return nil, errors.New("maximum value for disposable counter is 255")
	}

	c := &DisposableCounter{
		eternal: eternal,
	}
	c.Store(value)
	return c, nil
}

// Sub decreases counter.
func (d *DisposableCounter) Sub() {
	if d.eternal {
		return
	}

	for {
		old := d.Load()
		if old <= 0 {
			return
		}
		if d.CompareAndSwap(old, old-1) {
			return
		}
	}
}

// Eternal returns is DisposableCounter eternal.
func (d *DisposableCounter) Eternal() bool {
	return d.eternal
}

// Exhausted returns true if counter less then 1.
func (d *DisposableCounter) Exhausted() bool {
	if d.eternal {
		return false
	}
	return d.Load() < 1
}

// ClicksCounter record clicks counter.
type ClicksCounter struct {
	atomic.Uint32
}

// NewClicksCounter returns valid ClicksCounter.
func NewClicksCounter(value uint32) *ClicksCounter {
	cc := &ClicksCounter{}
	cc.Store(value)
	return cc
}

// Increase adds 1 to record clicks counter.
func (c *ClicksCounter) Increase() {
	c.Add(1)
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
