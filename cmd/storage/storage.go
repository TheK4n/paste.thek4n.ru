package storage

type KeysDB interface {
	Exists(string) (bool, error)
	Get(string) ([]byte, error)
	Set(string, []byte) error
}
