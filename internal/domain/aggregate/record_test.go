package aggregate

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thek4n/paste.thek4n.ru/internal/domain/domainerrors"
	"github.com/thek4n/paste.thek4n.ru/internal/domain/objectvalue"
)

func TestNewRecord(t *testing.T) {
	t.Run("new record with valid params returns record and no error", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(5 * time.Minute)
		body := []byte("test body")

		record, err := NewRecord("abc", expirationDate, 3, false, 0, body, false)

		require.NoError(t, err)
		assert.Equal(t, objectvalue.RecordKey("abc"), record.Key())
		assert.Equal(t, int32(3), record.DisposableCounter())
		assert.Equal(t, uint32(0), record.Clicks())
		assert.Equal(t, body, record.RGetBody())
		assert.False(t, record.URL())
	})
}

func TestRecord_GetBody(t *testing.T) {
	t.Run("get body returns body when record is valid", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(5 * time.Minute)
		body := []byte("hello")
		record, _ := NewRecord("key1", expirationDate, 2, false, 0, body, false)

		gotBody, err := record.GetBody()

		require.NoError(t, err)
		assert.Equal(t, body, gotBody)
		assert.Equal(t, uint32(1), record.Clicks())
		assert.Equal(t, int32(1), record.DisposableCounter())
	})

	t.Run("get body returns error when disposable counter is exhausted", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(5 * time.Minute)
		record, _ := NewRecord("key2", expirationDate, 1, false, 0, []byte("body"), false)

		// Exhaust counter
		_, _ = record.GetBody()

		// Now it should be exhausted
		_, err := record.GetBody()

		require.Error(t, err)
		assert.True(t, errors.Is(err, domainerrors.ErrRecordCounterExhausted))
	})

	t.Run("get body returns error when record is expired", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping time-sensitive test in short mode")
		}

		expirationDate := objectvalue.NewExpirationDateFromTTL(-1 * time.Minute)
		record, _ := NewRecord("key3", expirationDate, 5, false, 0, []byte("body"), false)

		_, err := record.GetBody()

		require.Error(t, err)
		assert.True(t, errors.Is(err, domainerrors.ErrRecordExpired))
	})

	t.Run("get body decreases disposable counter and increases clicks", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(5 * time.Minute)
		record, _ := NewRecord("key4", expirationDate, 3, false, 5, []byte("body"), false)

		_, err := record.GetBody()
		require.NoError(t, err)

		assert.Equal(t, int32(2), record.DisposableCounter())
		assert.Equal(t, uint32(6), record.Clicks())
	})
}

func TestRecord_RGetBody(t *testing.T) {
	t.Run("rget body returns body without checks", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(5 * time.Minute)
		body := []byte("secret")
		record, _ := NewRecord("key5", expirationDate, 1, false, 0, body, false)

		// Exhaust counter
		for range 2 {
			_, _ = record.GetBody()
		}

		// Even if exhausted, RGetBody should return body
		assert.Equal(t, body, record.RGetBody())
	})
}

func TestRecord_URL(t *testing.T) {
	t.Run("url returns correct value", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(5 * time.Minute)

		record1, _ := NewRecord("url1", expirationDate, 1, false, 0, []byte("https://example.com"), true)
		record2, _ := NewRecord("url2", expirationDate, 1, false, 0, []byte("plain text"), false)

		assert.True(t, record1.URL())
		assert.False(t, record2.URL())
	})
}

func TestRecord_TTL(t *testing.T) {
	t.Run("ttl returns correct duration until expiration", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(2 * time.Second)
		record, _ := NewRecord("key6", expirationDate, 1, false, 0, []byte("body"), false)

		ttl := record.TTL()
		assert.Greater(t, ttl, 1500*time.Millisecond)
		assert.Less(t, ttl, 2100*time.Millisecond)
	})

	t.Run("ttl returns zero or negative when expired", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping time-sensitive test in short mode")
		}

		expirationDate := objectvalue.NewExpirationDateFromTTL(-100 * time.Millisecond)
		record, _ := NewRecord("key7", expirationDate, 1, false, 0, []byte("body"), false)

		ttl := record.TTL()
		assert.LessOrEqual(t, ttl, 0*time.Second)
	})
}

func TestRecord_expired(t *testing.T) {
	t.Run("expired returns false for future expiration date", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(5 * time.Minute)
		record, _ := NewRecord("key8", expirationDate, 1, false, 0, []byte("body"), false)

		assert.False(t, record.expired())
	})

	t.Run("expired returns true for past expiration date", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(-1 * time.Second)
		record, _ := NewRecord("key9", expirationDate, 1, false, 0, []byte("body"), false)

		assert.True(t, record.expired())
	})

	t.Run("expired returns false for zero expiration date", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(0 * time.Second)
		record, _ := NewRecord("key9", expirationDate, 1, false, 0, []byte("body"), false)

		assert.False(t, record.expired())
	})
}

func TestRecord_counterExhausted(t *testing.T) {
	t.Run("counter exhausted returns false when counter > 0", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(1 * time.Hour)
		record, _ := NewRecord("key10", expirationDate, 1, false, 0, []byte("body"), false)

		assert.False(t, record.CounterExhausted())
	})

	t.Run("counter exhausted returns true when initial counter == 0", func(t *testing.T) {
		t.Parallel()

		expirationDate := objectvalue.NewExpirationDateFromTTL(1 * time.Hour)
		record, _ := NewRecord("key11", expirationDate, 0, false, 0, []byte("body"), false)

		assert.True(t, record.CounterExhausted())
	})
}
