package client

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu           sync.Mutex
	tokens       float64
	maxTokens    float64
	refillRate   float64 // tokens per second
	lastRefill   time.Time
}

// NewRateLimiter creates a new rate limiter.
// rate is the number of requests per second, burst is the maximum burst size.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: rate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a token is available or the context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()

		if rl.tokens >= 1 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}

		// Calculate wait time for next token
		tokensNeeded := 1 - rl.tokens
		waitTime := time.Duration(tokensNeeded/rl.refillRate*1000) * time.Millisecond
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue to retry
		}
	}
}

// refill adds tokens based on elapsed time (must be called with lock held).
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now
}

// Release returns a token to the bucket (for when requests fail early).
func (rl *RateLimiter) Release() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.tokens++
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
}
