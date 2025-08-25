package objectvalue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpirationDate(t *testing.T) {
	t.Run("new expiration date from ttl is in the future", func(t *testing.T) {
		t.Parallel()

		ttl := 5 * time.Second
		exp := NewExpirationDateFromTTL(ttl)

		assert.False(t, exp.Expired(), "expiration should not be expired immediately")
		assert.GreaterOrEqual(t, exp.Until(), 4*time.Second, "duration until expiration should be ~5s")
	})

	t.Run("expired returns true after duration passed", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping time-sensitive test in short mode")
		}

		exp := NewExpirationDateFromTTL(100 * time.Millisecond)

		time.Sleep(150 * time.Millisecond)

		assert.True(t, exp.Expired(), "expiration should be in the past")
		assert.Less(t, exp.Until(), 0*time.Second, "until should be negative")
	})

	t.Run("until returns correct duration", func(t *testing.T) {
		t.Parallel()

		ttl := 2 * time.Second
		exp := NewExpirationDateFromTTL(ttl)
		time.Sleep(500 * time.Millisecond)

		remaining := exp.Until()
		assert.Greater(t, remaining, 1*time.Second)
		assert.Less(t, remaining, 1500*time.Millisecond)
	})
}

func TestDisposableCounter(t *testing.T) {
	t.Run("new disposable counter with valid value returns counter", func(t *testing.T) {
		t.Parallel()

		counter, err := NewDisposableCounter(10, false)

		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, int32(10), counter.Load())
	})

	t.Run("new disposable counter with value > 255 returns error", func(t *testing.T) {
		t.Parallel()

		counter, err := NewDisposableCounter(256, false)

		require.Error(t, err)
		assert.Nil(t, counter)
		assert.Equal(t, "maximum value for disposable counter is 255", err.Error())
	})

	t.Run("sub decreases counter by one", func(t *testing.T) {
		t.Parallel()

		counter, _ := NewDisposableCounter(3, false)

		counter.Sub()

		assert.Equal(t, int32(2), counter.Load())
	})

	t.Run("sub does nothing when counter is already zero", func(t *testing.T) {
		t.Parallel()

		counter, _ := NewDisposableCounter(0, false)

		counter.Sub()
		counter.Sub()

		assert.Equal(t, int32(0), counter.Load())
	})

	t.Run("exhausted returns false when counter > 0", func(t *testing.T) {
		t.Parallel()

		counter, _ := NewDisposableCounter(1, false)

		assert.False(t, counter.Exhausted())
	})

	t.Run("exhausted returns true when initial counter == 0", func(t *testing.T) {
		t.Parallel()

		counter, _ := NewDisposableCounter(0, false)

		assert.True(t, counter.Exhausted())
	})

	t.Run("exhausted returns true after sub to zero", func(t *testing.T) {
		t.Parallel()

		counter, _ := NewDisposableCounter(1, false)
		counter.Sub()

		assert.True(t, counter.Exhausted())
	})

	t.Run("sub is safe under concurrent calls", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		counter, _ := NewDisposableCounter(100, false)
		workers := 10
		done := make(chan bool)

		for range workers {
			go func() {
				for range 10 {
					counter.Sub()
				}
				done <- true
			}()
		}

		for range workers {
			<-done
		}

		assert.Equal(t, int32(0), counter.Load())
		assert.True(t, counter.Exhausted())
	})
}

func TestClicksCounter(t *testing.T) {
	t.Run("new clicks counter initializes with given value", func(t *testing.T) {
		t.Parallel()

		counter := NewClicksCounter(5)

		assert.Equal(t, uint32(5), counter.Load())
	})

	t.Run("increase increments counter by one", func(t *testing.T) {
		t.Parallel()

		counter := NewClicksCounter(0)

		counter.Increase()

		assert.Equal(t, uint32(1), counter.Load())
	})

	t.Run("increase is safe for concurrent use", func(t *testing.T) {
		t.Parallel()

		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}

		counter := NewClicksCounter(0)
		incs := 100
		workers := 10
		done := make(chan bool)

		for range workers {
			go func() {
				for j := 0; j < incs/workers; j++ {
					counter.Increase()
				}
				done <- true
			}()
		}

		for range workers {
			<-done
		}

		assert.Equal(t, uint32(incs), counter.Load())
	})
}
