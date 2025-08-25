package objectvalue

import (
	"errors"
	"sync/atomic"
)

// QuotaSourceIP quota source ip type.
type QuotaSourceIP string

// Quota for requests by ip.
type Quota struct {
	value    *atomic.Int32
	_default int32
}

// NewQuota validate defaultValue and returns new quota.
func NewQuota(defaultValue int32) (*Quota, error) {
	if defaultValue < 1 {
		return nil, errors.New("invalid default quota")
	}
	q := &Quota{
		value:    &atomic.Int32{},
		_default: defaultValue,
	}
	q.Refresh()
	return q, nil
}

// Sub reduces quota by 1.
func (q *Quota) Sub() {
	q.value.Add(-1)
}

// Exhausted returns true if quota less then 1.
func (q *Quota) Exhausted() bool {
	return q.value.Load() < 1
}

// Refresh set default quota.
func (q *Quota) Refresh() {
	q.value.Store(q._default)
}

// Value returns value of quota.
func (q *Quota) Value() int32 {
	return q.value.Load()
}
