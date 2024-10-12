package database

import (
	"database/sql"
	"fmt"
	"time"

	"gpgenie/internal/config"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func Connect(cfg config.DatabaseConfig) (*sql.DB, error) {
	var db *sql.DB
	var err error

	switch cfg.Type {
	case "postgres":
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
		db, err = sql.Open("postgres", connStr)
	case "sqlite":
		db, err = sql.Open("sqlite3", cfg.DBName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool parameters
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// Verify connection
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// CloseDB closes the database connection
func CloseDB(db *sql.DB) {
	if db != nil {
		db.Close()
	}
}
