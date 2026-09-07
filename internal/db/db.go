package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect builds a pgx connection pool from the same environment variables the
// Spring Boot backend uses, so the existing ConfigMap/Secret work unchanged:
//
//	DB_HOST (default localhost), DB_PORT (default 5432), DB_NAME (default todo),
//	DB_USERNAME (default todo), DB_PASSWORD (REQUIRED — no default).
//
// DB_PASSWORD has no default on purpose (matching the hardened Spring config):
// the app fails fast if it is not provided.
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	name := envOrDefault("DB_NAME", "todo")
	user := envOrDefault("DB_USERNAME", "todo")
	password, ok := os.LookupEnv("DB_PASSWORD")
	if !ok || password == "" {
		return nil, fmt.Errorf("DB_PASSWORD must be set")
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, name,
	)

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connectivity at startup (fail fast, like Spring on bad DB config).
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}

func envOrDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
