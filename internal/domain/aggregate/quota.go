package aggregate

import (
	"fmt"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

// Quota aggregate.
type Quota struct {
	sourceIP objectvalue.QuotaSourceIP
	quota    *objectvalue.Quota
}

// NewQuota constructor.
func NewQuota(ip objectvalue.QuotaSourceIP, def int32) (Quota, error) {
	q, err := objectvalue.NewQuota(def)
	if err != nil {
		return Quota{}, fmt.Errorf("fail to create new quota counter: %w", err)
	}
	return Quota{
		sourceIP: ip,
		quota:    q,
	}, nil
}

// Exhausted is quota exhausted.
func (q Quota) Exhausted() bool {
	return q.quota.Exhausted()
}

// Refresh refreshes quota.
func (q *Quota) Refresh() {
	q.quota.Refresh()
}

// Sub sub -1 from quota.
func (q *Quota) Sub() {
	q.quota.Sub()
}

// SourceIP getter.
func (q Quota) SourceIP() objectvalue.QuotaSourceIP {
	return q.sourceIP
}

// Value getter.
func (q Quota) Value() int32 {
	return q.quota.Value()
}
