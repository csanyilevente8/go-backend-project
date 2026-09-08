package model

import (
	"time"

	"github.com/google/uuid"
)

// Activity is one entry in the activity_log — a record derived from a Kafka
// TodoEvent by the consumer, and returned by GET /api/activity.
type Activity struct {
	ID        int64     `json:"id"`
	TodoID    uuid.UUID `json:"todoId"`
	Type      string    `json:"type"`
	Detail    *string   `json:"detail"`
	CreatedAt time.Time `json:"createdAt"`
}
