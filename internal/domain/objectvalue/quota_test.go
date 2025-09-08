//go:build unit

package objectvalue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewQuota(t *testing.T) {
	t.Run("new quota with positive default value initializes with correct value", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(10)

		assert.Equal(t, uint32(10), quota.Value())
		assert.False(t, quota.Exhausted())
	})
}

func TestQuota_Sub(t *testing.T) {
	t.Run("sub reduces quota by one", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(3)

		quota = quota.Sub()

		assert.Equal(t, uint32(2), quota.Value())
	})

	t.Run("sub cant reduce quota below zero", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(1)

		quota = quota.Sub()

		assert.Equal(t, uint32(0), quota.Value())
		assert.True(t, quota.Exhausted())

		quota.Sub()

		assert.Equal(t, uint32(0), quota.Value())
		assert.True(t, quota.Exhausted())
	})
}

func TestQuota_Exhausted(t *testing.T) {
	t.Run("exhausted returns false when quota is above zero", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(1)

		assert.False(t, quota.Exhausted())
	})

	t.Run("exhausted returns true when quota is zero", func(t *testing.T) {
		t.Parallel()

		q := NewQuota(1)
		q = q.Sub()

		assert.True(t, q.Exhausted())
	})

	t.Run("exhausted returns true when quota is below zero", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(1)
		quota = quota.Sub()
		quota = quota.Sub()

		assert.True(t, quota.Exhausted())
	})
}

func TestQuota_Refresh(t *testing.T) {
	t.Run("refresh sets quota back to default value", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(5)

		quota = quota.Sub()
		quota = quota.Sub()

		assert.Equal(t, uint32(3), quota.Value())
		assert.False(t, quota.Exhausted())

		quota = quota.Refresh()

		assert.Equal(t, uint32(5), quota.Value())
		assert.False(t, quota.Exhausted())
	})

	t.Run("refresh after exhaustion restores quota", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(2)

		quota = quota.Sub()
		quota = quota.Sub()

		assert.True(t, quota.Exhausted())

		quota = quota.Refresh()

		assert.Equal(t, uint32(2), quota.Value())
		assert.False(t, quota.Exhausted())
	})
}

func TestQuota_Value(t *testing.T) {
	t.Run("value returns current quota value", func(t *testing.T) {
		t.Parallel()

		quota := NewQuota(10)

		assert.Equal(t, uint32(10), quota.Value())

		quota = quota.Sub()

		assert.Equal(t, uint32(9), quota.Value())
	})
}
