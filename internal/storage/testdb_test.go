package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestDB_Integration(t *testing.T) {
	ctx := context.Background()

	testDB, err := NewTestDB(ctx, t, "../../migrations")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer testDB.Close(ctx)

	t.Logf("Test DB created successfully with connection string: %s", testDB.ConnStr)
}

func TestDB_PgVectorExtension(t *testing.T) {
	ctx := context.Background()

	testDB, err := NewTestDB(ctx, t, "../../migrations")
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer testDB.Close(ctx)

	// Подключаемся к БД для проверки расширения pgvector
	db, err := sql.Open("pgx", testDB.ConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to test DB: %v", err)
	}
	defer db.Close()

	// Проверяем, что расширение pgvector установлено
	var extName string
	err = db.QueryRow("SELECT extname FROM pg_extension WHERE extname = 'vector'").Scan(&extName)
	if err != nil {
		t.Fatalf("pgvector extension not found: %v", err)
	}
	t.Logf("pgvector extension found: %s", extName)

	// Проверяем, что тип данных vector доступен
	var typname string
	err = db.QueryRow("SELECT typname FROM pg_type WHERE typname = 'vector'").Scan(&typname)
	if err != nil {
		t.Fatalf("vector data type not found: %v", err)
	}
	t.Logf("vector data type found: %s", typname)

	// Проверяем, что функции pgvector доступны
	var funcName string
	err = db.QueryRow("SELECT proname FROM pg_proc WHERE proname = 'vector_dims'").Scan(&funcName)
	if err != nil {
		t.Fatalf("vector_dims function not found: %v", err)
	}
	t.Logf("vector_dims function found: %s", funcName)

	// Тестируем создание вектора
	var result string
	err = db.QueryRow("SELECT '[1,2,3]'::vector").Scan(&result)
	if err != nil {
		t.Fatalf("Failed to create vector: %v", err)
	}
	t.Logf("Vector creation test passed: %s", result)
}
