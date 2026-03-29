package storage

import (
	"context"

	"github.com/mrPTqp/total-recaller/internal/models"
)

// MeetingRepository defines the interface for meeting storage operations
type MeetingRepository interface {
	Create(ctx context.Context, meeting *models.Meeting) error
	GetByID(ctx context.Context, id int, telegramID int64) (*models.Meeting, error)
	ListByUser(ctx context.Context, telegramID int64, limit int) ([]models.Meeting, error)
	Search(ctx context.Context, telegramID int64, query string, limit, offset int) ([]models.Meeting, error)
	UpdateSummary(ctx context.Context, telegramID int64, fileId string, summary string) error
	UpdateEmbedding(ctx context.Context, telegramID int64, fileId string, embedding models.Vector) error
	SearchByEmbedding(ctx context.Context, telegramID int64, queryEmbedding models.Vector, limit, offset int) ([]models.Meeting, error)
}
