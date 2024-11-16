// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package resilience

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	MaxRetries     = 3
	InitialBackoff = 100 * time.Millisecond
	MaxBackoff     = 2 * time.Second
)

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	failures     int
	lastFailure  time.Time
	mutex        sync.RWMutex
	maxFailures  int
	resetTimeout time.Duration
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) IsOpen() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	if cb.failures >= cb.maxFailures {
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			// Reset circuit breaker after timeout
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			cb.failures = 0
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return false
		}
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures = 0
}

// RetryWithBackoff implements exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, fn func() error) error {
	var err error
	backoff := InitialBackoff

	for i := 0; i < MaxRetries; i++ {
		if err = fn(); err == nil {
			return nil
		}

		// Check if context is cancelled before sleeping
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Exponential backoff with jitter
			jitter := time.Duration(float64(backoff) * (0.5 + rand.Float64())) // Add 50-150% jitter
			backoff = time.Duration(float64(backoff) * 2)
			if backoff > MaxBackoff {
				backoff = MaxBackoff
			}
			backoff += jitter
		}
	}

	return fmt.Errorf("failed after %d retries: %w", MaxRetries, err)
}
