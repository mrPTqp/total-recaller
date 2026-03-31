package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Import postgres driver
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDB представляет собой контейнерную БД для интеграционных тестов
type TestDB struct {
	container testcontainers.Container
	ConnStr   string
}

// NewTestDB создает новый контейнер с PostgreSQL и применяет миграции
func NewTestDB(ctx context.Context, t *testing.T, migrationsPath string) (*TestDB, error) {
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg18",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())

	testDB := &TestDB{
		container: container,
		ConnStr:   connStr,
	}

	err = testDB.Migrate(ctx, migrationsPath)
	if err != nil {
		testDB.Close(ctx)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return testDB, nil
}

func (tdb *TestDB) Close(ctx context.Context) error {
	return tdb.container.Terminate(ctx)
}

func (tdb *TestDB) Migrate(ctx context.Context, migrationsPath string) error {
	err := tdb.installPgVectorExtension(ctx)
	if err != nil {
		return fmt.Errorf("failed to install pgvector extension: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "goose-migrations")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	err = copyMigrations(migrationsPath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to copy migrations: %w", err)
	}

	err = runMigrations(tdb.ConnStr, tempDir)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func (tdb *TestDB) installPgVectorExtension(ctx context.Context) error {
	db, err := goose.OpenDBWithDriver("postgres", tdb.ConnStr)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	defer db.Close()

	var extName string
	err = db.QueryRow("SELECT extname FROM pg_extension WHERE extname = 'vector'").Scan(&extName)
	if err == nil {
		// Расширение уже установлено
		return nil
	}

	_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	return nil
}

func copyMigrations(src, dst string) error {
	files, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		srcPath := filepath.Join(src, file.Name())
		dstPath := filepath.Join(dst, file.Name())

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}

		err = os.WriteFile(dstPath, data, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func runMigrations(connStr, migrationsPath string) error {
	db, err := goose.OpenDBWithDriver("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	defer db.Close()

	err = goose.Up(db, migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// User represents a user in the database for testing purposes
type User struct {
	ID         int64
	TelegramID int64
	CreatedAt  time.Time
}

// GetUserByTelegramID retrieves a user by their Telegram ID
func (tdb *TestDB) GetUserByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	db, err := sql.Open("pgx", tdb.ConnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}
	defer db.Close()

	var user User
	err = db.QueryRowContext(ctx, "SELECT id, telegram_id, created_at FROM users WHERE telegram_id = $1", telegramID).
		Scan(&user.ID, &user.TelegramID, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateTestMeeting creates a test meeting for a user and returns the meeting ID
func (tdb *TestDB) CreateTestMeeting(ctx context.Context, telegramID int64, transcription string) (int, error) {
	db, err := sql.Open("pgx", tdb.ConnStr)
	if err != nil {
		return 0, fmt.Errorf("failed to open db: %w", err)
	}
	defer db.Close()

	// Ensure user exists
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (telegram_id, created_at) 
		VALUES ($1, $2) 
		ON CONFLICT (telegram_id) DO NOTHING`,
		telegramID, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to ensure user exists: %w", err)
	}

	// Create meeting
	var meetingID int
	err = db.QueryRowContext(ctx, `
		INSERT INTO meetings (telegram_id, file_id, full_text, created_at) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id`,
		telegramID, fmt.Sprintf("test_file_%d", time.Now().UnixNano()), transcription, time.Now()).
		Scan(&meetingID)
	if err != nil {
		return 0, fmt.Errorf("failed to create meeting: %w", err)
	}

	return meetingID, nil
}
