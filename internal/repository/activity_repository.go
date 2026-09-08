package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/csanyilevente8/go-backend-project/internal/model"
)

// ActivityRepository reads/writes the activity_log table (populated by the
// Kafka consumer, read by GET /api/activity).
type ActivityRepository struct {
	pool *pgxpool.Pool
}

func NewActivityRepository(pool *pgxpool.Pool) *ActivityRepository {
	return &ActivityRepository{pool: pool}
}

// Insert appends one activity entry (called by the event consumer).
func (r *ActivityRepository) Insert(ctx context.Context, todoID uuid.UUID, typ string, detail *string) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO activity_log (todo_id, type, detail) VALUES ($1,$2,$3)",
		todoID, typ, detail)
	return err
}

// FindRecent returns the most recent activity entries, newest first.
func (r *ActivityRepository) FindRecent(ctx context.Context, limit int) ([]model.Activity, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		"SELECT id, todo_id, type, detail, created_at FROM activity_log ORDER BY id DESC LIMIT $1",
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Activity, 0)
	for rows.Next() {
		var a model.Activity
		if err := rows.Scan(&a.ID, &a.TodoID, &a.Type, &a.Detail, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
