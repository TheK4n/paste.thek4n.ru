package aggregate

import (
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

// Quota aggregate.
type Quota struct {
	sourceIP objectvalue.QuotaSourceIP
	quota    objectvalue.Quota
}

// NewQuota constructor.
func NewQuota(ip objectvalue.QuotaSourceIP, def uint32) Quota {
	q := objectvalue.NewQuota(def)
	return Quota{
		sourceIP: ip,
		quota:    q,
	}
}

// Exhausted is quota exhausted.
func (q Quota) Exhausted() bool {
	return q.quota.Exhausted()
}

// Refresh refreshes quota.
func (q *Quota) Refresh() {
	q.quota = q.quota.Refresh()
}

// Sub sub -1 from quota.
func (q *Quota) Sub() {
	q.quota = q.quota.Sub()
}

// SourceIP getter.
func (q Quota) SourceIP() objectvalue.QuotaSourceIP {
	return q.sourceIP
}

// Value getter.
func (q Quota) Value() uint32 {
	return q.quota.Value()
}
