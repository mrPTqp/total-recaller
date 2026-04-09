package postgres

import (
	"context"
	"iter"
	"time"

	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/retry"
	"github.com/mrPTqp/total-recaller/internal/storage"
)

type retryMeetingRepository struct {
	repo        storage.MeetingRepository
	maxAttempts int
	backoff     time.Duration
	classifier  retry.ErrorClassifier
}

// NewRetryMeetingRepository creates a new meeting repository with retry logic
func NewRetryMeetingRepository(repo storage.MeetingRepository, maxAttempts int, backoff time.Duration) storage.MeetingRepository {
	return &retryMeetingRepository{
		repo:        repo,
		maxAttempts: maxAttempts,
		backoff:     backoff,
		classifier:  storage.NewPostgresErrorClassifier(),
	}
}

func (r *retryMeetingRepository) Create(ctx context.Context, meeting *models.Meeting) error {
	_, err := retry.Do(ctx, func(ctx context.Context) (any, error) {
		return nil, r.repo.Create(ctx, meeting)
	}, r.maxAttempts, r.backoff, r.classifier)
	return err
}

func (r *retryMeetingRepository) GetByID(ctx context.Context, id int, telegramID int64) (*models.Meeting, error) {
	return retry.Do(ctx, func(ctx context.Context) (*models.Meeting, error) {
		return r.repo.GetByID(ctx, id, telegramID)
	}, r.maxAttempts, r.backoff, r.classifier)
}

func (r *retryMeetingRepository) Search(ctx context.Context, telegramID int64, query string) iter.Seq[models.Meeting] {
	return r.repo.Search(ctx, telegramID, query)
}

func (r *retryMeetingRepository) UpdateSummary(ctx context.Context, telegramID int64, fileId string, summary string) error {
	_, err := retry.Do(ctx, func(ctx context.Context) (any, error) {
		return nil, r.repo.UpdateSummary(ctx, telegramID, fileId, summary)
	}, r.maxAttempts, r.backoff, r.classifier)
	return err
}

func (r *retryMeetingRepository) UpdateEmbedding(ctx context.Context, telegramID int64, fileId string, embedding models.Vector) error {
	_, err := retry.Do(ctx, func(ctx context.Context) (any, error) {
		return nil, r.repo.UpdateEmbedding(ctx, telegramID, fileId, embedding)
	}, r.maxAttempts, r.backoff, r.classifier)
	return err
}

func (r *retryMeetingRepository) SearchByEmbedding(ctx context.Context, telegramID int64, queryEmbedding models.Vector, limit, offset int) iter.Seq[models.Meeting] {
	return r.repo.SearchByEmbedding(ctx, telegramID, queryEmbedding, limit, offset)
}

func (r *retryMeetingRepository) ListByUser(ctx context.Context, telegramID int64) iter.Seq[models.Meeting] {
	return r.repo.ListByUser(ctx, telegramID)
}
