package storage

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompression(t *testing.T) {
	t.Parallel()

	t.Run("compress and decompress", func(t *testing.T) {
		data := []byte("test data to compress")
		compressed, err := compress(data)

		require.NoError(t, err)
		assert.True(t, isCompressed(compressed))

		decompressed, err := decompress(compressed)
		require.NoError(t, err)
		assert.Equal(t, data, decompressed)
	})

	t.Run("decompress invalid data", func(t *testing.T) {
		_, err := decompress([]byte("invalid gzip data"))
		require.Error(t, err)
	})
}

func FuzzIsCompressed(f *testing.F) {
	testCases := [][]byte{
		{0x1f, 0x8b},             // minimal valid case
		{0x1f, 0x8b, 0x08, 0x00}, // longer valid header
		{0x00, 0x00},             // invalid case
		{0x1f},                   // too short
		[]byte("0"),
		[]byte("\x1f0"),
		[]byte("\x0f0"),
		[]byte("\x89"),
		{}, // empty
	}

	for _, tc := range testCases {
		f.Add(tc)
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte("test data"))
	require.NoError(f, err)
	err = w.Close()
	require.NoError(f, err)
	f.Add(buf.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		result := isCompressed(data)

		if result {
			if len(data) < 2 {
				t.Errorf("isCompressed returned true for data shorter than 2 bytes: %v", data)
			}
			if data[0] != 0x1f || data[1] != 0x8b {
				t.Errorf("isCompressed returned true for data without gzip magic number: %v", data)
			}
		} else {
			if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
				t.Errorf("isCompressed returned false for valid gzip header: %v", data)
			}
		}
	})
}
