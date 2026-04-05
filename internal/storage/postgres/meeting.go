package postgres

import (
	"context"
	"fmt"
	"iter"

	"github.com/jmoiron/sqlx"
	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
)

type postgresMeetingRepository struct {
	db                    *sqlx.DB
	createStmt            *sqlx.Stmt
	getByIDStmt           *sqlx.Stmt
	listByUserStmt        *sqlx.Stmt
	searchStmt            *sqlx.Stmt
	updateEmbeddingStmt   *sqlx.Stmt
	searchByEmbeddingStmt *sqlx.Stmt
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
		INSERT INTO meetings (telegram_id, file_id, full_text, summary, embedding, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare create statement: %w", err))
	}

	r.getByIDStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, file_id, full_text, summary, embedding, created_at
		FROM meetings 
		WHERE id = $1 AND telegram_id = $2`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare get by ID statement: %w", err))
	}

	r.listByUserStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, file_id, full_text, summary, embedding, created_at
		FROM meetings 
		WHERE telegram_id = $1
		ORDER BY created_at DESC`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare list by user statement: %w", err))
	}

	r.searchStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, file_id, full_text, summary, embedding, created_at
		FROM meetings 
		WHERE telegram_id = $1 AND full_text_tsvector @@ plainto_tsquery('russian', $2)
		ORDER BY created_at DESC`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare search statement: %w", err))
	}

	r.updateEmbeddingStmt, err = r.db.Preparex(`
		UPDATE meetings 
		SET embedding = $1 
		WHERE telegram_id = $2 AND file_id = $3`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare update embedding statement: %w", err))
	}

	r.searchByEmbeddingStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, file_id, full_text, summary, embedding, created_at
		FROM meetings 
		WHERE telegram_id = $1
		ORDER BY embedding <=> $2`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare search by embedding statement: %w", err))
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
		meeting.Summary, meeting.Embedding, meeting.CreatedAt).Scan(&meeting.ID)
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

func (r *postgresMeetingRepository) Search(ctx context.Context, telegramID int64, query string) iter.Seq[models.Meeting] {
	return func(yield func(models.Meeting) bool) {
		rows, err := r.searchStmt.QueryxContext(ctx, telegramID, query)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var meeting models.Meeting
			if err := rows.StructScan(&meeting); err != nil {
				return
			}
			if !yield(meeting) {
				return
			}
		}
	}
}

func (r *postgresMeetingRepository) UpdateSummary(ctx context.Context, telegramID int64, fileId string, summary string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE meetings 
		SET summary = $1 
		WHERE telegram_id = $2 AND file_id = $3`,
		summary, telegramID, fileId)
	if err != nil {
		return fmt.Errorf("failed to update meeting summary: %w", err)
	}

	return tx.Commit()
}

func (r *postgresMeetingRepository) UpdateEmbedding(ctx context.Context, telegramID int64, fileId string, embedding models.Vector) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = r.updateEmbeddingStmt.ExecContext(ctx, embedding, telegramID, fileId)
	if err != nil {
		return fmt.Errorf("failed to update meeting embedding: %w", err)
	}

	return tx.Commit()
}

func (r *postgresMeetingRepository) SearchByEmbedding(ctx context.Context, telegramID int64, queryEmbedding models.Vector, limit, offset int) iter.Seq[models.Meeting] {
	return func(yield func(models.Meeting) bool) {
		rows, err := r.searchByEmbeddingStmt.QueryxContext(ctx, telegramID, queryEmbedding)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var meeting models.Meeting
			if err := rows.StructScan(&meeting); err != nil {
				return
			}
			if !yield(meeting) {
				return
			}
		}
	}
}

func (r *postgresMeetingRepository) ListByUser(ctx context.Context, telegramID int64) iter.Seq[models.Meeting] {
	return func(yield func(models.Meeting) bool) {
		rows, err := r.listByUserStmt.QueryxContext(ctx, telegramID)
		if err != nil {
			return
		}
		defer rows.Close()

		for rows.Next() {
			var meeting models.Meeting
			if err := rows.StructScan(&meeting); err != nil {
				return
			}
			if !yield(meeting) {
				return
			}
		}
	}
}
