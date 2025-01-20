package storage

type KeysDB interface {
	// Get key from db. It returns nil, nil if key not exists
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Exists(string) (bool, error)
}
