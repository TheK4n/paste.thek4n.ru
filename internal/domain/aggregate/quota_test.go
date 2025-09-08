//go:build unit

package aggregate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

func TestNewQuota(t *testing.T) {
	t.Run("new quota with valid default value returns quota", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.1")
		quota := NewQuota(ip, 5)

		assert.Equal(t, ip, quota.SourceIP())
		assert.Equal(t, uint32(5), quota.Value())
		assert.False(t, quota.Exhausted())
	})
}

func TestQuota_Exhausted(t *testing.T) {
	t.Run("exhausted returns false when quota is above zero", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.1")
		quota := NewQuota(ip, 1)

		assert.False(t, quota.Exhausted())
	})

	t.Run("exhausted returns true when quota is zero", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.2")
		quota := NewQuota(ip, 1)
		quota.Sub()

		assert.True(t, quota.Exhausted())
	})

	t.Run("exhausted returns true when quota is below zero", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.3")
		quota := NewQuota(ip, 2)
		quota.Sub()
		quota.Sub()
		quota.Sub() // goes to -1

		assert.True(t, quota.Exhausted())
	})
}

func TestQuota_Refresh(t *testing.T) {
	t.Run("refresh sets quota back to default value", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.10")
		quota := NewQuota(ip, 10)

		quota.Sub()
		quota.Sub()

		assert.Equal(t, uint32(8), quota.Value())
		assert.False(t, quota.Exhausted())

		quota.Refresh()

		assert.Equal(t, uint32(10), quota.Value())
		assert.False(t, quota.Exhausted())
	})
}

func TestQuota_Sub(t *testing.T) {
	t.Run("sub decreases quota by one", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.20")
		quota := NewQuota(ip, 3)

		quota.Sub()

		assert.Equal(t, uint32(2), quota.Value())
	})

	t.Run("sub cant reduce quota below zero", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.21")
		quota := NewQuota(ip, 1)
		quota.Sub()

		assert.Equal(t, uint32(0), quota.Value())
		quota.Sub()

		assert.Equal(t, uint32(0), quota.Value())
	})
}

func TestQuota_SourceIP(t *testing.T) {
	t.Run("source ip returns correct ip", func(t *testing.T) {
		t.Parallel()

		expectedIP := objectvalue.QuotaSourceIP("203.0.113.42")
		quota := NewQuota(expectedIP, 5)

		assert.Equal(t, expectedIP, quota.SourceIP())
	})
}

func TestQuota_Value(t *testing.T) {
	t.Run("value returns current quota value", func(t *testing.T) {
		t.Parallel()

		ip := objectvalue.QuotaSourceIP("192.168.1.30")
		quota := NewQuota(ip, 7)

		assert.Equal(t, uint32(7), quota.Value())

		quota.Sub()
		assert.Equal(t, uint32(6), quota.Value())

		quota.Refresh()
		assert.Equal(t, uint32(7), quota.Value())
	})
}
