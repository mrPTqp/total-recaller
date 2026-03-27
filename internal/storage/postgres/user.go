package postgres

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/mrPTqp/total-recaller/internal/models"
	"github.com/mrPTqp/total-recaller/internal/storage"
)

type postgresUserRepository struct {
	db           *sqlx.DB
	createStmt   *sqlx.Stmt
	getByTgIDStmt *sqlx.Stmt
}

// NewUserRepository creates a new PostgreSQL user repository
func NewUserRepository(db *sqlx.DB) storage.UserRepository {
	repo := &postgresUserRepository{db: db}
	repo.prepareStatements()
	return repo
}

func (r *postgresUserRepository) prepareStatements() {
	var err error
	
	r.createStmt, err = r.db.Preparex(`
		INSERT INTO users (telegram_id, username, first_name, last_name, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name
		RETURNING id`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare create statement: %w", err))
	}

	r.getByTgIDStmt, err = r.db.Preparex(`
		SELECT id, telegram_id, username, first_name, last_name, created_at
		FROM users 
		WHERE telegram_id = $1`)
	if err != nil {
		panic(fmt.Errorf("failed to prepare get by telegram ID statement: %w", err))
	}
}

func (r *postgresUserRepository) Create(ctx context.Context, user *models.User) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	err = r.createStmt.QueryRowContext(ctx,
		user.TelegramID, user.Username, user.FirstName, user.LastName, user.CreatedAt).Scan(&user.ID)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return tx.Commit()
}

func (r *postgresUserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	var user models.User
	err := r.getByTgIDStmt.GetContext(ctx, &user, telegramID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by telegram ID: %w", err)
	}

	return &user, nil
}
