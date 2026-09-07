package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/csanyilevente8/go-backend-project/internal/model"
)

// ErrNotFound is returned when a todo does not exist (maps to HTTP 404).
var ErrNotFound = errors.New("todo not found")

// TodoRepository is the data-access layer for the todos table (which already
// exists — created by the Spring Boot app's Flyway migration; this app does not
// manage the schema).
type TodoRepository struct {
	pool *pgxpool.Pool
}

func NewTodoRepository(pool *pgxpool.Pool) *TodoRepository {
	return &TodoRepository{pool: pool}
}

const columns = "id, title, description, completed, created_at, updated_at"

func (r *TodoRepository) FindAll(ctx context.Context) ([]model.Todo, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT "+columns+" FROM todos ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	todos := make([]model.Todo, 0)
	for rows.Next() {
		var t model.Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Completed, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

func (r *TodoRepository) FindByID(ctx context.Context, id uuid.UUID) (model.Todo, error) {
	var t model.Todo
	err := r.pool.QueryRow(ctx,
		"SELECT "+columns+" FROM todos WHERE id = $1", id).
		Scan(&t.ID, &t.Title, &t.Description, &t.Completed, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Todo{}, ErrNotFound
	}
	return t, err
}

func (r *TodoRepository) Create(ctx context.Context, title string, description *string) (model.Todo, error) {
	now := time.Now().UTC()
	t := model.Todo{
		ID:          uuid.New(),
		Title:       title,
		Description: description,
		Completed:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := r.pool.Exec(ctx,
		"INSERT INTO todos ("+columns+") VALUES ($1,$2,$3,$4,$5,$6)",
		t.ID, t.Title, t.Description, t.Completed, t.CreatedAt, t.UpdatedAt)
	return t, err
}

func (r *TodoRepository) Update(ctx context.Context, id uuid.UUID, title string, description *string, completed bool) (model.Todo, error) {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		"UPDATE todos SET title=$1, description=$2, completed=$3, updated_at=$4 WHERE id=$5",
		title, description, completed, now, id)
	if err != nil {
		return model.Todo{}, err
	}
	if tag.RowsAffected() == 0 {
		return model.Todo{}, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *TodoRepository) SetCompleted(ctx context.Context, id uuid.UUID, completed bool) (model.Todo, error) {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		"UPDATE todos SET completed=$1, updated_at=$2 WHERE id=$3",
		completed, now, id)
	if err != nil {
		return model.Todo{}, err
	}
	if tag.RowsAffected() == 0 {
		return model.Todo{}, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *TodoRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM todos WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
