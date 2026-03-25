package postgres

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
)

type postgresMeetingRepository struct {
	db             *sqlx.DB
	createStmt     *sqlx.Stmt
	getByIDStmt    *sqlx.Stmt
	listByUserStmt *sqlx.Stmt
	searchStmt     *sqlx.Stmt
}

// NewMeetingRepository creates a new PostgreSQL meeting repository
func NewMeetingRepository(db *sqlx.DB) storage.MeetingRepository {
	repo := &postgresMeetingRepository{db: db}
	repo.prepareStatements()
	return repo
}

func (r *postgresMeetingRepository) prepareStatements() {
	var err error

	r.createStmt, err = r.db.Preparex(`
		INSERT INTO meetings (telegram_id, file_id, full_text, summary, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare create statement: %w", err))
	}

	r.getByIDStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, file_id, full_text, summary, created_at
		FROM meetings 
		WHERE id = $1 AND telegram_id = $2`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare get by ID statement: %w", err))
	}

	r.listByUserStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, file_id, full_text, summary, created_at
		FROM meetings 
		WHERE telegram_id = $1
		ORDER BY created_at DESC 
		LIMIT $2`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare list by user statement: %w", err))
	}

	r.searchStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, file_id, full_text, summary, created_at
		FROM meetings 
		WHERE telegram_id = $1 AND full_text_tsvector @@ plainto_tsquery('russian', $2)
		ORDER BY created_at DESC 
		LIMIT $3 OFFSET $4`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare search statement: %w", err))
	}
}

func (r *postgresMeetingRepository) Create(ctx context.Context, meeting *models.Meeting) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	err = r.createStmt.QueryRowContext(ctx,
		meeting.TelegramID, meeting.FileId, meeting.FullText,
		meeting.Summary, meeting.CreatedAt).Scan(&meeting.ID)
	if err != nil {
		return fmt.Errorf("failed to create meeting: %w", err)
	}

	return tx.Commit()
}

func (r *postgresMeetingRepository) GetByID(ctx context.Context, id int, telegramID int64) (*models.Meeting, error) {
	var meeting models.Meeting
	err := r.getByIDStmt.GetContext(ctx, &meeting, id, telegramID)
	if err != nil {
		return nil, fmt.Errorf("failed to get meeting by ID: %w", err)
	}

	return &meeting, nil
}

func (r *postgresMeetingRepository) ListByUser(ctx context.Context, telegramID int64, limit int) ([]models.Meeting, error) {
	var meetings []models.Meeting
	err := r.listByUserStmt.SelectContext(ctx, &meetings, telegramID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list meetings: %w", err)
	}

	return meetings, nil
}

func (r *postgresMeetingRepository) Search(ctx context.Context, telegramID int64, query string, limit, offset int) ([]models.Meeting, error) {
	var meetings []models.Meeting
	err := r.searchStmt.SelectContext(ctx, &meetings, telegramID, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search meetings: %w", err)
	}

	return meetings, nil
}
