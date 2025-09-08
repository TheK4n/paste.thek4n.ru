// Package repository contains interfaces of repositories.
package repository

import (
	"context"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/aggregate"
)

// APIKeyRORepository readonly interface for apikeys.
type APIKeyRORepository interface {
	GetByID(context.Context, string) (aggregate.APIKey, error)
	GetAll(context.Context) ([]aggregate.APIKey, error)
	Exists(context.Context, string) (bool, error)
}

// APIKeyWORepository write interface for apikeys.
type APIKeyWORepository interface {
	SetByID(context.Context, string, aggregate.APIKey) error
	RemoveByID(context.Context, string) error
}
