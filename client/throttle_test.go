package client

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSlidingWindowThrottle(t *testing.T) {
	t.Run("allows requests under the limit", func(t *testing.T) {
		throttle := NewSlidingWindowThrottle(10)
		ctx := context.Background()

		// Should allow 10 requests immediately
		for i := 0; i < 10; i++ {
			err := throttle.Acquire(ctx)
			if err != nil {
				t.Errorf("Acquire() error = %v, want nil", err)
			}
		}

		if throttle.GetWindowCount() != 10 {
			t.Errorf("GetWindowCount() = %d, want 10", throttle.GetWindowCount())
		}
	})

	t.Run("GetRemaining returns correct value", func(t *testing.T) {
		throttle := NewSlidingWindowThrottle(10)
		ctx := context.Background()

		if throttle.GetRemaining() != 10 {
			t.Errorf("GetRemaining() = %d, want 10", throttle.GetRemaining())
		}

		for i := 0; i < 5; i++ {
			throttle.Acquire(ctx)
		}

		if throttle.GetRemaining() != 5 {
			t.Errorf("GetRemaining() = %d, want 5", throttle.GetRemaining())
		}
	})

	t.Run("Reset clears the window", func(t *testing.T) {
		throttle := NewSlidingWindowThrottle(10)
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			throttle.Acquire(ctx)
		}

		if throttle.GetWindowCount() != 5 {
			t.Errorf("GetWindowCount() = %d, want 5", throttle.GetWindowCount())
		}

		throttle.Reset()

		if throttle.GetWindowCount() != 0 {
			t.Errorf("GetWindowCount() after Reset() = %d, want 0", throttle.GetWindowCount())
		}
	})

	t.Run("defaults to 100 for invalid values", func(t *testing.T) {
		throttle := NewSlidingWindowThrottle(0)
		if throttle.requestsPer10Seconds != 100 {
			t.Errorf("requestsPer10Seconds = %d, want 100", throttle.requestsPer10Seconds)
		}

		throttle2 := NewSlidingWindowThrottle(-5)
		if throttle2.requestsPer10Seconds != 100 {
			t.Errorf("requestsPer10Seconds = %d, want 100", throttle2.requestsPer10Seconds)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		throttle := NewSlidingWindowThrottle(1)
		ctx := context.Background()

		// Use up the one request slot
		throttle.Acquire(ctx)

		// Create a cancelled context
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		// Should return immediately with context error
		err := throttle.Acquire(cancelCtx)
		if err != context.Canceled {
			t.Errorf("Acquire() error = %v, want context.Canceled", err)
		}
	})

	t.Run("is thread-safe", func(t *testing.T) {
		throttle := NewSlidingWindowThrottle(100)
		ctx := context.Background()

		var wg sync.WaitGroup
		errors := make(chan error, 50)

		// Spawn 50 goroutines each making 1 request
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := throttle.Acquire(ctx); err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent Acquire() error = %v", err)
		}

		// All 50 requests should be tracked
		count := throttle.GetWindowCount()
		if count != 50 {
			t.Errorf("GetWindowCount() = %d, want 50", count)
		}
	})
}

func TestSlidingWindowThrottleBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping blocking test in short mode")
	}

	t.Run("blocks when limit is reached", func(t *testing.T) {
		throttle := NewSlidingWindowThrottle(2)
		ctx := context.Background()

		// Use up both slots
		throttle.Acquire(ctx)
		throttle.Acquire(ctx)

		// Create a context with timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// This should block and timeout
		err := throttle.Acquire(timeoutCtx)
		if err != context.DeadlineExceeded {
			t.Errorf("Acquire() error = %v, want context.DeadlineExceeded", err)
		}
	})
}

func TestNoOpThrottle(t *testing.T) {
	t.Run("Acquire returns immediately", func(t *testing.T) {
		throttle := NewNoOpThrottle()
		ctx := context.Background()

		// Should allow unlimited requests
		for i := 0; i < 1000; i++ {
			err := throttle.Acquire(ctx)
			if err != nil {
				t.Errorf("Acquire() error = %v, want nil", err)
			}
		}
	})

	t.Run("GetWindowCount returns 0", func(t *testing.T) {
		throttle := NewNoOpThrottle()
		if throttle.GetWindowCount() != 0 {
			t.Errorf("GetWindowCount() = %d, want 0", throttle.GetWindowCount())
		}
	})

	t.Run("GetRemaining returns large number", func(t *testing.T) {
		throttle := NewNoOpThrottle()
		if throttle.GetRemaining() != 1000000 {
			t.Errorf("GetRemaining() = %d, want 1000000", throttle.GetRemaining())
		}
	})

	t.Run("Reset does not panic", func(t *testing.T) {
		throttle := NewNoOpThrottle()
		throttle.Reset() // Should not panic
	})
}

func TestThrottleInterface(t *testing.T) {
	// Verify both types implement Throttle interface
	var _ Throttle = (*SlidingWindowThrottle)(nil)
	var _ Throttle = (*NoOpThrottle)(nil)
}
