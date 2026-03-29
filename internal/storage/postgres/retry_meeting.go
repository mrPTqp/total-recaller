package postgres

import (
	"context"
	"time"

	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
)

type retryMeetingRepository struct {
	repo        storage.MeetingRepository
	maxAttempts int
	backoff     time.Duration
}

// NewRetryMeetingRepository creates a new meeting repository with retry logic
func NewRetryMeetingRepository(repo storage.MeetingRepository, maxAttempts int, backoff time.Duration) storage.MeetingRepository {
	return &retryMeetingRepository{
		repo:        repo,
		maxAttempts: maxAttempts,
		backoff:     backoff,
	}
}

func (r *retryMeetingRepository) Create(ctx context.Context, meeting *models.Meeting) error {
	_, err := storage.WithRetryContext(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, r.repo.Create(ctx, meeting)
	}, r.maxAttempts, r.backoff)
	return err
}

func (r *retryMeetingRepository) GetByID(ctx context.Context, id int, telegramID int64) (*models.Meeting, error) {
	return storage.WithRetryContext(ctx, func(ctx context.Context) (*models.Meeting, error) {
		return r.repo.GetByID(ctx, id, telegramID)
	}, r.maxAttempts, r.backoff)
}

func (r *retryMeetingRepository) ListByUser(ctx context.Context, telegramID int64, limit int) ([]models.Meeting, error) {
	return storage.WithRetryContext(ctx, func(ctx context.Context) ([]models.Meeting, error) {
		return r.repo.ListByUser(ctx, telegramID, limit)
	}, r.maxAttempts, r.backoff)
}

func (r *retryMeetingRepository) Search(ctx context.Context, telegramID int64, query string, limit, offset int) ([]models.Meeting, error) {
	return storage.WithRetryContext(ctx, func(ctx context.Context) ([]models.Meeting, error) {
		return r.repo.Search(ctx, telegramID, query, limit, offset)
	}, r.maxAttempts, r.backoff)
}

func (r *retryMeetingRepository) UpdateSummary(ctx context.Context, telegramID int64, fileId string, summary string) error {
	_, err := storage.WithRetryContext(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, r.repo.UpdateSummary(ctx, telegramID, fileId, summary)
	}, r.maxAttempts, r.backoff)
	return err
}

func (r *retryMeetingRepository) UpdateEmbedding(ctx context.Context, telegramID int64, fileId string, embedding []float32) error {
	_, err := storage.WithRetryContext(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, r.repo.UpdateEmbedding(ctx, telegramID, fileId, embedding)
	}, r.maxAttempts, r.backoff)
	return err
}

func (r *retryMeetingRepository) SearchByEmbedding(ctx context.Context, telegramID int64, queryEmbedding []float32, limit, offset int) ([]models.Meeting, error) {
	return storage.WithRetryContext(ctx, func(ctx context.Context) ([]models.Meeting, error) {
		return r.repo.SearchByEmbedding(ctx, telegramID, queryEmbedding, limit, offset)
	}, r.maxAttempts, r.backoff)
}

