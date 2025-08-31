package objectvalue

// QuotaSourceIP quota source ip type.
type QuotaSourceIP string

// Quota for requests by ip.
type Quota struct {
	value    uint32
	_default uint32
}

// NewQuota validate defaultValue and returns new quota.
func NewQuota(defaultValue uint32) Quota {
	q := Quota{
		value:    defaultValue,
		_default: defaultValue,
	}
	return q
}

// Sub reduces quota by 1.
func (q Quota) Sub() Quota {
	if q.value == 0 {
		return q
	}
	q.value--
	return q
}

// Exhausted returns true if quota less then 1.
func (q Quota) Exhausted() bool {
	return q.value < 1
}

// Refresh set default quota.
func (q Quota) Refresh() Quota {
	q.value = q._default
	return q
}

// Value returns value of quota.
func (q Quota) Value() uint32 {
	return q.value
}
