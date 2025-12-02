package client

import (
	"context"
	"sync"
	"time"
)

// Throttle is the interface for rate limiting strategies.
type Throttle interface {
	// Acquire blocks until a request slot is available.
	Acquire(ctx context.Context) error
	// GetWindowCount returns the number of requests in the current window.
	GetWindowCount() int
	// GetRemaining returns remaining requests available in the current window.
	GetRemaining() int
	// Reset clears the throttle state.
	Reset()
}

// SlidingWindowThrottle implements QuickBase's rate limit: 100 requests per 10 seconds.
// This matches the JS SDK's SlidingWindowThrottle.
type SlidingWindowThrottle struct {
	mu                   sync.Mutex
	requestsPer10Seconds int
	timestamps           []time.Time
}

// NewSlidingWindowThrottle creates a new sliding window throttle.
// Default is 100 requests per 10 seconds (QuickBase's limit).
func NewSlidingWindowThrottle(requestsPer10Seconds int) *SlidingWindowThrottle {
	if requestsPer10Seconds <= 0 {
		requestsPer10Seconds = 100
	}
	return &SlidingWindowThrottle{
		requestsPer10Seconds: requestsPer10Seconds,
		timestamps:           make([]time.Time, 0, requestsPer10Seconds),
	}
}

// Acquire waits until a request slot is available.
func (t *SlidingWindowThrottle) Acquire(ctx context.Context) error {
	for {
		t.mu.Lock()
		now := time.Now()
		windowStart := now.Add(-10 * time.Second)

		// Remove timestamps outside the window
		newTimestamps := t.timestamps[:0]
		for _, ts := range t.timestamps {
			if ts.After(windowStart) {
				newTimestamps = append(newTimestamps, ts)
			}
		}
		t.timestamps = newTimestamps

		// If under the limit, record this request and return
		if len(t.timestamps) < t.requestsPer10Seconds {
			t.timestamps = append(t.timestamps, now)
			t.mu.Unlock()
			return nil
		}

		// Calculate wait time until oldest request exits the window
		oldestTimestamp := t.timestamps[0]
		waitTime := oldestTimestamp.Add(10 * time.Second).Sub(now)
		t.mu.Unlock()

		if waitTime <= 0 {
			continue // Recheck immediately
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue to recheck
		}
	}
}

// GetWindowCount returns the number of requests in the current 10-second window.
func (t *SlidingWindowThrottle) GetWindowCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	windowStart := time.Now().Add(-10 * time.Second)
	count := 0
	for _, ts := range t.timestamps {
		if ts.After(windowStart) {
			count++
		}
	}
	return count
}

// GetRemaining returns remaining requests available in the current window.
func (t *SlidingWindowThrottle) GetRemaining() int {
	return max(0, t.requestsPer10Seconds-t.GetWindowCount())
}

// Reset clears the throttle state.
func (t *SlidingWindowThrottle) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timestamps = t.timestamps[:0]
}

// NoOpThrottle is a throttle that does nothing (for when throttling is disabled).
type NoOpThrottle struct{}

// NewNoOpThrottle creates a no-op throttle.
func NewNoOpThrottle() *NoOpThrottle {
	return &NoOpThrottle{}
}

// Acquire does nothing and returns immediately.
func (t *NoOpThrottle) Acquire(ctx context.Context) error {
	return nil
}

// GetWindowCount always returns 0.
func (t *NoOpThrottle) GetWindowCount() int {
	return 0
}

// GetRemaining always returns a large number.
func (t *NoOpThrottle) GetRemaining() int {
	return 1000000
}

// Reset does nothing.
func (t *NoOpThrottle) Reset() {}
