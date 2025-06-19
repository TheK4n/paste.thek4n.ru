package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompression(t *testing.T) {
	t.Run("compress and decompress", func(t *testing.T) {
		data := []byte("test data to compress")

		compressed, err := compress(data)

		assert.NoError(t, err)
		assert.True(t, isCompressed(compressed))

		decompressed, err := decompress(compressed)
		assert.NoError(t, err)
		assert.Equal(t, data, decompressed)
	})

	t.Run("decompress invalid data", func(t *testing.T) {
		_, err := decompress([]byte("invalid gzip data"))
		assert.Error(t, err)
	})
}
