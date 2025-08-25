// Package aggregate contains domain aggregates
package aggregate

import (
	"fmt"
	"time"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

// Record represents cached record.
type Record struct {
	key               objectvalue.RecordKey
	expirationDate    objectvalue.ExpirationDate
	disposableCounter *objectvalue.DisposableCounter
	clicks            *objectvalue.ClicksCounter
	body              []byte
	url               bool
}

// NewRecord creates Record with initialized params.
func NewRecord(
	key string,
	expirationDate time.Time,
	disposableCounter uint8,
	eternal bool,
	clicks uint32,
	body []byte,
	url bool,
) (Record, error) {
	c, err := objectvalue.NewDisposableCounter(int32(disposableCounter), eternal)
	if err != nil {
		return Record{}, fmt.Errorf("fail to create counter: %w", err)
	}

	cc := objectvalue.NewClicksCounter(clicks)

	return Record{
		key:               objectvalue.RecordKey(key),
		expirationDate:    objectvalue.ExpirationDate(expirationDate),
		disposableCounter: c,
		body:              body,
		clicks:            cc,
		url:               url,
	}, nil
}

// Key record key getter.
func (r Record) Key() objectvalue.RecordKey {
	return r.key
}

// Clicks record clicks getter.
func (r Record) Clicks() uint32 {
	return r.clicks.Load()
}

// DisposableCounter record disposable counter getter.
func (r Record) DisposableCounter() int32 {
	return r.disposableCounter.Load()
}

// DisposableCounterEternal returns is disposableCounter is eternal.
func (r Record) DisposableCounterEternal() bool {
	return r.disposableCounter.Eternal()
}

// GetBody checks is record counter exhausted, is record expired.
// Then decreases disposable counter, increases clicks counter and returns body.
func (r *Record) GetBody() ([]byte, error) {
	if r.counterExhausted() {
		return nil, domainerrors.ErrRecordCounterExhausted
	}

	if r.expired() {
		return nil, domainerrors.ErrRecordExpired
	}

	r.decreaseDisposableCounter()

	r.increaseClicksCounter()

	return r.body, nil
}

// RGetBody body getter.
func (r Record) RGetBody() []byte {
	return r.body
}

// URL getter.
func (r Record) URL() bool {
	return r.url
}

// TTL returns duration until record expiration date.
func (r *Record) TTL() time.Duration {
	return r.expirationDate.Until()
}

func (r *Record) expired() bool {
	return r.expirationDate.Expired()
}

func (r *Record) counterExhausted() bool {
	return r.disposableCounter.Exhausted()
}

func (r *Record) decreaseDisposableCounter() {
	r.disposableCounter.Sub()
}

func (r *Record) increaseClicksCounter() {
	r.clicks.Increase()
}
