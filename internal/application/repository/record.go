package repository

import (
	"context"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

// RecordRepository domain interface.
type RecordRepository interface {
	GetByKey(context.Context, objectvalue.RecordKey) (aggregate.Record, error)
	SetByKey(context.Context, objectvalue.RecordKey, aggregate.Record) error
	Exists(context.Context, objectvalue.RecordKey) (bool, error)
	GenerateUniqueKey(ctx context.Context, minLength uint8, maxLength uint8) (objectvalue.RecordKey, error)
}
