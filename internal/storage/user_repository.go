package storage

import (
	"context"

	"github.com/mrPTqp/total-recaller/internal/models"
)

// UserRepository defines the interface for user storage operations
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error)
}
