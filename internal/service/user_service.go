package service

import (
	"context"
	"fmt"

	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
	"go.uber.org/zap"
)

type UserService struct {
	repo   storage.UserRepository
	logger *zap.Logger
}

func NewUserService(repo storage.UserRepository, logger *zap.Logger) *UserService {
	return &UserService{
		repo:   repo,
		logger: logger,
	}
}

func (s *UserService) RegisterUser(
	ctx context.Context,
	telegramID int64,
	username, firstName, lastName string,
) error {
	user := &models.User{
		TelegramID: telegramID,
		Username:   username,
		FirstName:  firstName,
		LastName:   lastName,
		CreatedAt:  models.TimeNow(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to register user", zap.Int64("telegram_id", telegramID), zap.Error(err))
		return fmt.Errorf("failed to register user: %w", err)
	}

	return nil
}
