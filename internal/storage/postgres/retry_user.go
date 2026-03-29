package postgres

import (
	"context"
	"time"

	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
)

type retryUserRepository struct {
	repo        storage.UserRepository
	maxAttempts int
	backoff     time.Duration
}

// NewRetryUserRepository creates a new user repository with retry logic
func NewRetryUserRepository(repo storage.UserRepository, maxAttempts int, backoff time.Duration) storage.UserRepository {
	return &retryUserRepository{
		repo:        repo,
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

func (r *retryUserRepository) Create(ctx context.Context, user *models.User) error {
	_, err := storage.WithRetryContext(ctx, func(ctx context.Context) (any, error) {
		return nil, r.repo.Create(ctx, user)
	}, r.maxAttempts, r.backoff)
	return err
}

func (r *retryUserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	return storage.WithRetryContext(ctx, func(ctx context.Context) (*models.User, error) {
		return r.repo.GetByTelegramID(ctx, telegramID)
	}, r.maxAttempts, r.backoff)
}
