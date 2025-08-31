// Package aggregate contains domain aggregates
package aggregate

import (
	"time"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

// Record represents cached record.
type Record struct {
	key               objectvalue.RecordKey
	expirationDate    objectvalue.ExpirationDate
	disposableCounter objectvalue.DisposableCounter
	clicks            objectvalue.ClicksCounter
	body              []byte
	url               bool
}

// NewRecord creates Record with initialized params.
func NewRecord(
	key string,
	expirationDate objectvalue.ExpirationDate,
	disposableCounter uint8,
	eternal bool,
	clicks uint32,
	body []byte,
	url bool,
) Record {
	c := objectvalue.NewDisposableCounter(disposableCounter, eternal)
	cc := objectvalue.NewClicksCounter(clicks)

	return Record{
		key:               objectvalue.RecordKey(key),
		expirationDate:    expirationDate,
		disposableCounter: c,
		body:              body,
		clicks:            cc,
		url:               url,
	}
}

// Key record key getter.
func (r Record) Key() objectvalue.RecordKey {
	return r.key
}

// Clicks record clicks getter.
func (r Record) Clicks() uint32 {
	return r.clicks.Value()
}

// DisposableCounter record disposable counter getter.
func (r Record) DisposableCounter() uint8 {
	return r.disposableCounter.Value()
}

// DisposableCounterEternal returns is disposableCounter is eternal.
func (r Record) DisposableCounterEternal() bool {
	return r.disposableCounter.Eternal()
}

// ExpirationDateEternal returns is expirationDate is eternal.
func (r Record) ExpirationDateEternal() bool {
	return r.expirationDate.Eternal()
}

// GetBody checks is record counter exhausted, is record expired.
// Then decreases disposable counter, increases clicks counter and returns body.
func (r *Record) GetBody() ([]byte, error) {
	if r.CounterExhausted() {
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

// CounterExhausted returns true if disposable counter equal or less then 0.
func (r *Record) CounterExhausted() bool {
	return r.disposableCounter.Exhausted()
}

func (r *Record) decreaseDisposableCounter() {
	r.disposableCounter = r.disposableCounter.Sub()
}

func (r *Record) increaseClicksCounter() {
	r.clicks = r.clicks.Increase()
}
