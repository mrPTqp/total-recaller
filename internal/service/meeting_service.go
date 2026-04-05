package service

import (
	"context"
	"fmt"
	"iter"

	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
	"go.uber.org/zap"
)

type MeetingService struct {
	repo     storage.MeetingRepository
	userRepo storage.UserRepository
	logger   *zap.Logger
}

func NewMeetingService(repo storage.MeetingRepository, userRepo storage.UserRepository, logger *zap.Logger) *MeetingService {
	return &MeetingService{
		repo:     repo,
		userRepo: userRepo,
		logger:   logger,
	}
}

func (s *MeetingService) CreateMeeting(ctx context.Context, telegramID int64, fileId, transcription string) (*models.Meeting, error) {
	// First, ensure the user exists in the database
	_, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		// User doesn't exist, create a new user record
		user := &models.User{
			TelegramID: telegramID,
			CreatedAt:  models.TimeNow(),
		}

		createErr := s.userRepo.Create(ctx, user)
		if createErr != nil {
			s.logger.Error("Failed to create user before meeting", zap.Int64("telegram_id", telegramID), zap.Error(createErr))
			return nil, fmt.Errorf("failed to create user: %w", createErr)
		}
		s.logger.Info("Created new user", zap.Int64("telegram_id", telegramID))
	}

	meeting := &models.Meeting{
		TelegramID: telegramID,
		FileId:     fileId,
		CreatedAt:  models.TimeNow(),
		FullText:   transcription,
	}

	if err := s.repo.Create(ctx, meeting); err != nil {
		s.logger.Error("Failed to create meeting", zap.Error(err))
		return nil, fmt.Errorf("failed to create meeting: %w", err)
	}

	return meeting, nil
}

func (s *MeetingService) GetMeeting(ctx context.Context, id int, telegramID int64) (*models.Meeting, error) {
	meeting, err := s.repo.GetByID(ctx, id, telegramID)
	if err != nil {
		s.logger.Error("Failed to get meeting", zap.Int("id", id), zap.Error(err))
		return nil, fmt.Errorf("failed to get meeting: %w", err)
	}

	return meeting, nil
}

func (s *MeetingService) ListMeetings(ctx context.Context, telegramID int64) iter.Seq[models.Meeting] {
	return s.repo.ListByUser(ctx, telegramID)
}

func (s *MeetingService) SearchMeetings(ctx context.Context, telegramID int64, query string) iter.Seq[models.Meeting] {
	return s.repo.Search(ctx, telegramID, query)
}

func (s *MeetingService) UpdateMeetingSummary(ctx context.Context, telegramID int64, fileId string, summary string) error {
	err := s.repo.UpdateSummary(ctx, telegramID, fileId, summary)
	if err != nil {
		s.logger.Error("Failed to update meeting summary", zap.String("file_id", fileId), zap.Error(err))
		return fmt.Errorf("failed to update meeting summary: %w", err)
	}

	return nil
}

func (s *MeetingService) UpdateMeetingEmbedding(ctx context.Context, telegramID int64, fileId string, embedding []float32) error {
	err := s.repo.UpdateEmbedding(ctx, telegramID, fileId, models.FromFloat32Slice(embedding))
	if err != nil {
		s.logger.Error("Failed to update meeting embedding", zap.String("file_id", fileId), zap.Error(err))
		return fmt.Errorf("failed to update meeting embedding: %w", err)
	}

	return nil
}

func (s *MeetingService) SearchMeetingsByEmbedding(ctx context.Context, telegramID int64, queryEmbedding []float32, limit, offset int) iter.Seq[models.Meeting] {
	return s.repo.SearchByEmbedding(ctx, telegramID, models.FromFloat32Slice(queryEmbedding), limit, offset)
}
