package event

import (
	"github.com/thek4n/paste.thek4n.ru/pkg/apikeys"
)

// APIKeyUsedEvent describes apikey usage.
type APIKeyUsedEvent struct {
	baseEvent
	apikeyID string
	reason   apikeys.UsageReason
	fromIP   string
}

// NewAPIKeyUsedEvent constructor.
func NewAPIKeyUsedEvent(
	apikeyID string,
	reason apikeys.UsageReason,
	fromIP string,
) APIKeyUsedEvent {
	baseEvent := baseEvent{
		name:            "usagereason.new",
		isAsynchronious: true,
	}
	return APIKeyUsedEvent{
		baseEvent: baseEvent,
		apikeyID:  apikeyID,
		reason:    reason,
		fromIP:    fromIP,
	}
}

// Reason getter.
func (e APIKeyUsedEvent) Reason() apikeys.UsageReason {
	return e.reason
}

// APIKeyID getter.
func (e APIKeyUsedEvent) APIKeyID() string {
	return e.apikeyID
}

// FromIP getter.
func (e APIKeyUsedEvent) FromIP() string {
	return e.fromIP
}
