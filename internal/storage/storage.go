package storage

import (
	"errors"
)

var (
	ErrKeyNotFound = errors.New("Key not found in db")
)

type KeysDB interface {
	// Returns nil, storage.ErrKeyNotFound if key not found
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Exists(string) (bool, error)
}
