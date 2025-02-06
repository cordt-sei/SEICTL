package common

import (
	"context"
	"time"
)

// RetryOptions defines retry behavior
type RetryOptions struct {
	MaxAttempts int
	Delay       time.Duration
}

// RetryWithContext executes function with retries
func RetryWithContext(ctx context.Context, opts RetryOptions, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < opts.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := fn(); err == nil {
				return nil
			} else {
				lastErr = err
				time.Sleep(opts.Delay)
			}
		}
	}
	return lastErr
}

// DefaultRetryOptions returns default retry settings
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts: 3,
		Delay:       time.Second * 5,
	}
}
