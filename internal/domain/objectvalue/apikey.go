package objectvalue

import (
	"fmt"

	"github.com/google/uuid"
)

// APIKeyID id for api key.
type APIKeyID uuid.UUID

// NewAPIKeyID consructor.
func NewAPIKeyID(k string) (APIKeyID, error) {
	u, err := uuid.Parse(k)
	if err != nil {
		return APIKeyID{}, fmt.Errorf("fail to parse key: %w", err)
	}

	return APIKeyID(u), nil
}

func (k APIKeyID) String() string {
	return uuid.UUID(k).String()
}

// NilAPIKeyID nil apikey id.
var NilAPIKeyID = APIKeyID(uuid.Nil)
