package objectvalue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQuota(t *testing.T) {
	t.Run("new quota with valid default value returns quota and no error", func(t *testing.T) {
		t.Parallel()

		quota, err := NewQuota(5)

		require.NoError(t, err)
		assert.NotNil(t, quota)
		assert.Equal(t, int32(5), quota.Value())
	})

	t.Run("new quota with zero default value returns error", func(t *testing.T) {
		t.Parallel()

		quota, err := NewQuota(0)

		require.Error(t, err)
		assert.Nil(t, quota)
		assert.Equal(t, "invalid default quota", err.Error())
	})

	t.Run("new quota with negative default value returns error", func(t *testing.T) {
		t.Parallel()

		quota, err := NewQuota(-3)

		require.Error(t, err)
		assert.Nil(t, quota)
		assert.Equal(t, "invalid default quota", err.Error())
	})

	t.Run("new quota with positive default value initializes with correct value", func(t *testing.T) {
		t.Parallel()

		quota, err := NewQuota(10)
		require.NoError(t, err)

		assert.Equal(t, int32(10), quota.Value())
		assert.False(t, quota.Exhausted())
	})
}

func TestQuota_Sub(t *testing.T) {
	t.Run("sub reduces quota by one", func(t *testing.T) {
		t.Parallel()

		quota, _ := NewQuota(3)

		quota.Sub()

		assert.Equal(t, int32(2), quota.Value())
	})

	t.Run("sub can reduce quota below zero", func(t *testing.T) {
		t.Parallel()

		quota, _ := NewQuota(1)

		quota.Sub()

		assert.Equal(t, int32(0), quota.Value())
		assert.True(t, quota.Exhausted())

		quota.Sub()

		assert.Equal(t, int32(-1), quota.Value())
		assert.True(t, quota.Exhausted())
	})
}

func TestQuota_Exhausted(t *testing.T) {
	t.Run("exhausted returns false when quota is above zero", func(t *testing.T) {
		t.Parallel()

		quota, _ := NewQuota(1)

		assert.False(t, quota.Exhausted())
	})

	t.Run("exhausted returns true when quota is zero", func(t *testing.T) {
		t.Parallel()

		q, _ := NewQuota(1)
		q.Sub()

		assert.True(t, q.Exhausted())
	})

	t.Run("exhausted returns true when quota is below zero", func(t *testing.T) {
		t.Parallel()

		quota, _ := NewQuota(1)
		quota.Sub()
		quota.Sub()

		assert.True(t, quota.Exhausted())
		assert.Equal(t, int32(-1), quota.Value())
	})
}

func TestQuota_Refresh(t *testing.T) {
	t.Run("refresh sets quota back to default value", func(t *testing.T) {
		t.Parallel()

		quota, _ := NewQuota(5)

		quota.Sub()
		quota.Sub()

		assert.Equal(t, int32(3), quota.Value())
		assert.False(t, quota.Exhausted())

		quota.Refresh()

		assert.Equal(t, int32(5), quota.Value())
		assert.False(t, quota.Exhausted())
	})

	t.Run("refresh after exhaustion restores quota", func(t *testing.T) {
		t.Parallel()

		quota, _ := NewQuota(2)

		quota.Sub()
		quota.Sub()

		assert.True(t, quota.Exhausted())

		quota.Refresh()

		assert.Equal(t, int32(2), quota.Value())
		assert.False(t, quota.Exhausted())
	})
}

func TestQuota_Value(t *testing.T) {
	t.Run("value returns current quota value", func(t *testing.T) {
		t.Parallel()

		quota, _ := NewQuota(10)

		assert.Equal(t, int32(10), quota.Value())

		quota.Sub()

		assert.Equal(t, int32(9), quota.Value())
	})

	t.Run("value is safe for concurrent access", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		quota, _ := NewQuota(100)

		done := make(chan bool)
		subCount := 50

		for range 2 {
			go func() {
				for range subCount {
					quota.Sub()
				}
				done <- true
			}()
		}

		// Wait for both goroutines
		<-done
		<-done

		expected := int32(100 - 2*subCount)
		assert.Equal(t, expected, quota.Value())
	})
}
