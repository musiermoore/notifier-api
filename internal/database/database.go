package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/alexandersustavov/notifier/notifier-api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type DB struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*DB, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}

	db := &DB{pool: pool}
	if err := db.runMigrations(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) runMigrations(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		names = append(names, entry.Name())
	}

	sort.Strings(names)
	for _, name := range names {
		query, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.pool.Exec(ctx, string(query)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}

	return nil
}
