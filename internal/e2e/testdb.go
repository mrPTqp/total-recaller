package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Import postgres driver
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SharedPostgres holds the shared PostgreSQL container that is reused across all tests
type SharedPostgres struct {
	container testcontainers.Container
	host      string
	port      string
	mu        sync.Mutex
	ready     bool
}

// globalSharedPostgres is the singleton shared PostgreSQL instance
var globalSharedPostgres *SharedPostgres
var sharedPostgresOnce sync.Once

// getSharedPostgres returns the global shared PostgreSQL container, creating it if needed
func getSharedPostgres(ctx context.Context) (*SharedPostgres, error) {
	var initErr error
	sharedPostgresOnce.Do(func() {
		globalSharedPostgres, initErr = createSharedPostgres(ctx)
	})
	return globalSharedPostgres, initErr
}

// createSharedPostgres creates a new shared PostgreSQL container
func createSharedPostgres(ctx context.Context) (*SharedPostgres, error) {
	fmt.Println("🐳 Creating shared PostgreSQL container...")

	waitStrategy := wait.ForLog("database system is ready to accept connections").
		WithOccurrence(1).
		WithStartupTimeout(120 * time.Second)

	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg18",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "postgres",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
		},
		WaitingFor: waitStrategy,
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

	// Wait for the connection to be established before returning
	connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/postgres?sslmode=disable", host, port.Port())
	if err := waitForPostgresConnection(connStr, ctx); err != nil {
		return nil, fmt.Errorf("failed to establish connection to shared postgres: %w", err)
	}

	fmt.Println("✅ Shared PostgreSQL container is ready")

	return &SharedPostgres{
		container: container,
		host:      host,
		port:      port.Port(),
		ready:     true,
	}, nil
}

// waitForPostgresConnection waits for PostgreSQL to accept connections
func waitForPostgresConnection(connStr string, ctx context.Context) error {
	const maxRetries = 20
	const retryDelay = 500 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("failed to open connection: %w", err)
			}
			time.Sleep(retryDelay)
			continue
		}

		err = db.PingContext(ctx)
		db.Close()

		if err == nil {
			return nil
		}

		if attempt == maxRetries {
			return fmt.Errorf("failed to ping database: %w", err)
		}

		time.Sleep(retryDelay)
	}

	return fmt.Errorf("failed to establish connection after %d attempts", maxRetries)
}

// createTestDatabase creates a new database for a specific test
func (sp *SharedPostgres) createTestDatabase(ctx context.Context) (string, string, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Generate unique database name
	dbName := fmt.Sprintf("testdb_%d", time.Now().UnixNano())

	// Connect to the default postgres database
	adminConnStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/postgres?sslmode=disable", sp.host, sp.port)

	db, err := sql.Open("pgx", adminConnStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to open admin connection: %w", err)
	}
	defer db.Close()

	// Create the test database
	_, err = db.ExecContext(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
	if err != nil {
		return "", "", fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	// Build connection string for the new database
	connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/%s?sslmode=disable", sp.host, sp.port, dbName)

	return dbName, connStr, nil
}

// dropTestDatabase drops a test database
func (sp *SharedPostgres) dropTestDatabase(ctx context.Context, dbName string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	adminConnStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/postgres?sslmode=disable", sp.host, sp.port)

	db, err := sql.Open("pgx", adminConnStr)
	if err != nil {
		return fmt.Errorf("failed to open admin connection: %w", err)
	}
	defer db.Close()

	// Create a fresh context for cleanup operations to avoid context.Canceled errors
	// when the parent context has been canceled during test teardown
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Terminate all connections to the database first
	_, err = db.ExecContext(cleanupCtx, fmt.Sprintf(`
		SELECT pg_terminate_backend(pid) 
		FROM pg_stat_activity 
		WHERE datname = '%s' AND pid <> pg_backend_pid()`, dbName))
	if err != nil {
		// Log but don't fail - the database might not have active connections
		fmt.Printf("Warning: failed to terminate connections to %s: %v\n", dbName, err)
	}

	// Drop the database
	_, err = db.ExecContext(cleanupCtx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))
	if err != nil {
		return fmt.Errorf("failed to drop database %s: %w", dbName, err)
	}

	return nil
}

// stop stops the shared PostgreSQL container (called once at the end of all tests)
func (sp *SharedPostgres) stop(ctx context.Context) error {
	if sp.container != nil {
		fmt.Println("🛑 Stopping shared PostgreSQL container...")
		return sp.container.Terminate(ctx)
	}
	return nil
}

// TestDB represents a test database instance (uses shared container)
type TestDB struct {
	container testcontainers.Container // Reference to shared container
	ConnStr   string
	dbName    string
}

// NewTestDB creates a new test database using the shared PostgreSQL container
func NewTestDB(ctx context.Context, t *testing.T, migrationsPath string) (*TestDB, error) {
	shared, err := getSharedPostgres(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared postgres: %w", err)
	}

	// Create a unique database for this test
	dbName, connStr, err := shared.createTestDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create test database: %w", err)
	}

	testDB := &TestDB{
		container: shared.container,
		ConnStr:   connStr,
		dbName:    dbName,
	}

	// Wait for the new database to be ready
	err = testDB.waitForConnection(ctx)
	if err != nil {
		testDB.Close(ctx)
		return nil, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Apply migrations
	err = testDB.Migrate(ctx, migrationsPath)
	if err != nil {
		testDB.Close(ctx)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return testDB, nil
}

func (tdb *TestDB) Close(ctx context.Context) error {
	// Drop the test database
	if tdb.dbName != "" {
		shared := globalSharedPostgres
		if shared != nil {
			// Use a fresh context for cleanup to avoid context.Canceled errors
			// when the parent context has been canceled during test teardown
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			return shared.dropTestDatabase(cleanupCtx, tdb.dbName)
		}
	}
	return nil
}

// waitForConnection waits for the database connection to be established
func (tdb *TestDB) waitForConnection(ctx context.Context) error {
	const maxRetries = 10
	const retryDelay = 500 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		db, err := sql.Open("pgx", tdb.ConnStr)
		if err != nil {
			if attempt == maxRetries {
				return fmt.Errorf("failed to open db connection: %w", err)
			}
			time.Sleep(retryDelay * time.Duration(attempt))
			continue
		}

		err = db.PingContext(ctx)
		db.Close()

		if err == nil {
			return nil
		}

		if attempt == maxRetries {
			return fmt.Errorf("failed to ping database: %w", err)
		}

		time.Sleep(retryDelay * time.Duration(attempt))
	}

	return fmt.Errorf("failed to establish connection after %d attempts", maxRetries)
}

func (tdb *TestDB) Migrate(ctx context.Context, migrationsPath string) error {
	// Retry logic for handling transient connection issues
	const maxRetries = 3
	const retryDelay = 500 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = tdb.migrateOnce(ctx, migrationsPath)
		if lastErr == nil {
			return nil
		}

		if attempt < maxRetries {
			time.Sleep(retryDelay * time.Duration(attempt))
		}
	}

	return fmt.Errorf("failed to apply migrations after %d attempts: %w", maxRetries, lastErr)
}

func (tdb *TestDB) migrateOnce(ctx context.Context, migrationsPath string) error {
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
		return nil // Extension already installed
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