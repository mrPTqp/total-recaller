package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Do выполняет операцию с повторением при ошибках
func Do[T any](
	ctx context.Context,
	operation func(context.Context) (T, error),
	maxAttempts int,
	backoff time.Duration,
	classifier ErrorClassifier,
) (T, error) {
	var result T
	var allErrs []error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		default:
		}

		var err error
		result, err = operation(ctx)
		if err == nil {
			return result, nil
		}

		allErrs = append(allErrs, err)

		if classifier != nil && classifier.Classify(err) == NonRetriable {
			return result, err
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

	if len(allErrs) > 1 {
		return result, fmt.Errorf("operation failed after %d attempts: %w", maxAttempts, errors.Join(allErrs...))
	}
	return result, allErrs[0]
}

// ErrorClassifier интерфейс для классификации ошибок
type ErrorClassifier interface {
	Classify(error) ErrorClassification
}

// ErrorClassification определяет, следует ли повторять операцию
type ErrorClassification int

const (
	NonRetriable ErrorClassification = iota
	Retriable
)