// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package resilience

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

const (
	MaxRetries     = 3
	InitialBackoff = 100 * time.Millisecond
	MaxBackoff     = 2 * time.Second
)

// RetryWithBackoff implements exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, fn func() error) error {
	var err error
	backoff := InitialBackoff

	for range MaxRetries {
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
			backoff = min(time.Duration(float64(backoff)*2), MaxBackoff)
			backoff += jitter
		}
	}

	return fmt.Errorf("failed after %d retries: %w", MaxRetries, err)
}
