package storage

import (
	"context"
	"errors"
	"time"
)

var (
	ErrKeyNotFound = errors.New("Key not found in db")
)

type KeysDB interface {
	// Returns nil, storage.ErrKeyNotFound if key not found
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Exists(context.Context, string) (bool, error)
	Ping(context.Context) bool
}
