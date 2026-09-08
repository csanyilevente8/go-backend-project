package model

import (
	"time"

	"github.com/google/uuid"
)

// Notification is one entry in the notifications table — derived from a Kafka
// TodoEvent by the notifier consumer (consumer group "notifier"), independently
// of the activity consumer (fan-out). Returned by GET /api/notifications.
type Notification struct {
	ID        int64     `json:"id"`
	TodoID    uuid.UUID `json:"todoId"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}
