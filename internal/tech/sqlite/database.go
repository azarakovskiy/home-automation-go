package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"home-go/internal/config"
	_ "modernc.org/sqlite"
)

// Open opens (or creates) a SQLite database using the given DatabaseConfig and
// applies all pending migrations before returning.
func Open(cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer

	if err := runMigrations(db, cfg.MigrationsDir); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB, migrationsDir string) error {
	absDir, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("resolve migrations dir: %w", err)
	}

	src, err := source.Open("file://" + absDir)
	if err != nil {
		return fmt.Errorf("open migration source: %w", err)
	}

	driver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("file", src, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
