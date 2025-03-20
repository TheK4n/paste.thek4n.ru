package storage

import (
	"context"
	"errors"
	"time"
)

var (
	ErrKeyNotFound = errors.New("Key not found in db")
)

type Record struct {
	Disposable bool   `redis:"disposable"`
	URL        bool   `redis:"url"`
	Countdown  int    `redis:"countdown"`
	Clicks     int    `redis:"clicks"`
	Body       []byte `redis:"body"`
}

type RecordAnswer struct {
	URL  bool   `redis:"url"`
	Body []byte `redis:"body"`
}

type KeysDB interface {
	// Returns nil, storage.ErrKeyNotFound if key not found
	Get(context.Context, string) (RecordAnswer, error)
	// Returns nil, storage.ErrKeyNotFound if key not found
	GetClicks(context.Context, string) (int, error)
	Set(context.Context, string, time.Duration, Record) error
	Exists(context.Context, string) (bool, error)
	Ping(context.Context) bool
}
