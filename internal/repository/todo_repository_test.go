package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/csanyilevente8/go-backend-project/internal/repository"
)

// Full-stack integration test against a real PostgreSQL managed by
// Testcontainers, mirroring the Spring TodoIntegrationTest. Verifies the
// schema + CRUD end to end.
func TestTodoRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	ctx := context.Background()

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("todo"),
		postgres.WithUsername("todo"),
		postgres.WithPassword("todo"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// Create the schema (same DDL as the migration).
	_, err = pool.Exec(ctx, `CREATE TABLE todos (
		id UUID NOT NULL,
		title VARCHAR(255) NOT NULL,
		description VARCHAR(2000),
		completed BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		CONSTRAINT pk_todos PRIMARY KEY (id))`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	repo := repository.NewTodoRepository(pool)

	// Create
	desc := "port the backend"
	created, err := repo.Create(ctx, "Learn Go", &desc)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Completed {
		t.Fatal("new todo should not be completed")
	}

	// FindByID
	got, err := repo.FindByID(ctx, created.ID)
	if err != nil || got.Title != "Learn Go" {
		t.Fatalf("findByID: %v (%+v)", err, got)
	}

	// Update
	updated, err := repo.Update(ctx, created.ID, "Learn Go", strptr("and pgx"), true)
	if err != nil || !updated.Completed || *updated.Description != "and pgx" {
		t.Fatalf("update: %v (%+v)", err, updated)
	}

	// SetCompleted
	patched, err := repo.SetCompleted(ctx, created.ID, false)
	if err != nil || patched.Completed {
		t.Fatalf("setCompleted: %v (%+v)", err, patched)
	}

	// FindAll
	all, err := repo.FindAll(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("findAll: %v (len=%d)", err, len(all))
	}

	// Delete
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.FindByID(ctx, created.ID); err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Not found paths
	if err := repo.Delete(ctx, created.ID); err != repository.ErrNotFound {
		t.Fatalf("expected ErrNotFound deleting missing, got %v", err)
	}
}

func strptr(s string) *string { return &s }
