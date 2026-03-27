package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Database struct {
	db *sqlx.DB
}

func NewDatabase(dsn string, maxOpenConns, maxIdleConns int, maxLifetime, maxIdleTime time.Duration) (*Database, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(maxOpenConns)       
	db.SetMaxIdleConns(maxIdleConns)     
	db.SetConnMaxLifetime(maxLifetime)     
	db.SetConnMaxIdleTime(maxIdleTime)     

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) GetDB() *sqlx.DB {
	return d.db
}

func (d *Database) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}
