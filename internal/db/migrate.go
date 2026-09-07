package db

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies pending SQL migrations from the embedded migrations/
// directory, like Flyway does for the Spring backend. Idempotent: safe to run
// on every startup and safe against a schema already created by Flyway (the
// initial migration uses CREATE TABLE IF NOT EXISTS).
//
// Note on shared ownership: while both backends run against the same database,
// Flyway (Spring) remains the primary schema owner. golang-migrate keeps its
// own tracking table (schema_migrations); keep the DDL identical across both.
func RunMigrations(pool *pgxpool.Pool) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}

	// golang-migrate's postgres driver needs a *sql.DB; derive one from the
	// pool's config via the pgx stdlib adapter.
	sqlDB := stdlib.OpenDBFromPool(pool)
	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
