package storage

import (
	"context"
	"time"
)

// WithRetryContext executes a function with retry logic and context
func WithRetryContext[T any](ctx context.Context, fn func(context.Context) (T, error), maxAttempts int, backoff time.Duration) (T, error) {
	var result T
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		default:
			result, lastErr = fn(ctx)
			if lastErr == nil {
				return result, nil
			}
		}

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			case <-time.After(backoff * time.Duration(attempt)):
			}
		}
	}

	return result, lastErr
}
