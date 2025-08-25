package aggregate

import (
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

// APIKey aggregate.
type APIKey struct {
	key      string
	publicID objectvalue.APIKeyID
	valid    bool
}

// NewAPIKey constructor.
func NewAPIKey(publicID objectvalue.APIKeyID, key string, valid bool) APIKey {
	return APIKey{
		key:      key,
		publicID: publicID,
		valid:    valid,
	}
}

// PublicID getter for apikey id.
func (a APIKey) PublicID() objectvalue.APIKeyID {
	return a.publicID
}

// Valid getter.
func (a APIKey) Valid() bool {
	return a.valid
}

// Key getter.
func (a APIKey) Key() string {
	return a.key
}

// Invalidate invalidates apikey.
func (a *APIKey) Invalidate() {
	a.valid = false
}

// Reauthorize reauthorizes apikey.
func (a *APIKey) Reauthorize() {
	a.valid = true
}
