package service

import (
	"context"
	"fmt"

	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
	"go.uber.org/zap"
)

type MeetingService struct {
	repo   storage.MeetingRepository
	logger *zap.Logger
}

func NewMeetingService(repo storage.MeetingRepository, logger *zap.Logger) *MeetingService {
	return &MeetingService{
		repo:   repo,
		logger: logger,
	}
}

func (s *MeetingService) CreateMeeting(ctx context.Context, telegramID int64, fileId, transcription string) (*models.Meeting, error) {
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

func (s *MeetingService) ListMeetings(ctx context.Context, telegramID int64) ([]models.Meeting, error) {
	meetings, err := s.repo.ListByUser(ctx, telegramID, 100)
	if err != nil {
		s.logger.Error("Failed to list meetings", zap.Error(err))
		return nil, fmt.Errorf("failed to list meetings: %w", err)
	}

	return meetings, nil
}

func (s *MeetingService) SearchMeetings(ctx context.Context, telegramID int64, query string, limit, offset int) ([]models.Meeting, error) {
	meetings, err := s.repo.Search(ctx, telegramID, query, limit, offset)
	if err != nil {
		s.logger.Error("Failed to search meetings", zap.String("query", query), zap.Error(err))
		return nil, fmt.Errorf("failed to search meetings: %w", err)
	}

	return meetings, nil
}

func (s *MeetingService) UpdateMeetingSummary(ctx context.Context, telegramID int64, fileId string, summary string) error {
	err := s.repo.UpdateSummary(ctx, telegramID, fileId, summary)
	if err != nil {
		s.logger.Error("Failed to update meeting summary", zap.String("file_id", fileId), zap.Error(err))
		return fmt.Errorf("failed to update meeting summary: %w", err)
	}

	return nil
}
