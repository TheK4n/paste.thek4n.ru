package repository

import (
	"context"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

// QuotaRepository repository interface.
type QuotaRepository interface {
	GetByID(context.Context, objectvalue.QuotaSourceIP) (aggregate.Quota, error)
	SetByID(context.Context, objectvalue.QuotaSourceIP, aggregate.Quota) error
}
