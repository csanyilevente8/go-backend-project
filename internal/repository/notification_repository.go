package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/csanyilevente8/go-backend-project/internal/model"
)

// NotificationRepository reads/writes the notifications table (populated by the
// "notifier" Kafka consumer, read by GET /api/notifications). It is fed by a
// consumer group independent of the activity_log — the same todo-events feed
// both, demonstrating Kafka fan-out.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

// Insert appends one notification (called by the notifier consumer).
func (r *NotificationRepository) Insert(ctx context.Context, todoID uuid.UUID, typ, message string) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO notifications (todo_id, type, message) VALUES ($1,$2,$3)",
		todoID, typ, message)
	return err
}

// FindRecent returns the most recent notifications, newest first.
func (r *NotificationRepository) FindRecent(ctx context.Context, limit int) ([]model.Notification, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		"SELECT id, todo_id, type, message, read, created_at FROM notifications ORDER BY id DESC LIMIT $1",
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Notification, 0)
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.TodoID, &n.Type, &n.Message, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CountUnread returns the number of unread notifications (for the UI badge).
func (r *NotificationRepository) CountUnread(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM notifications WHERE read = false").Scan(&n)
	return n, err
}

// MarkAllRead marks every notification as read (called when the user opens the
// notifications view). Returns the number of rows updated.
func (r *NotificationRepository) MarkAllRead(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, "UPDATE notifications SET read = true WHERE read = false")
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
