package events

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Topic is the Kafka topic all todo domain events are published to.
const Topic = "todo-events"

// Event types (shared contract with the Spring backend and any consumer).
const (
	TypeCreated   = "TodoCreated"
	TypeUpdated   = "TodoUpdated"
	TypeCompleted = "TodoCompleted"
	TypeDeleted   = "TodoDeleted"
)

// TodoEvent is the message published to Kafka after a successful DB mutation.
// The JSON shape is the cross-service contract:
//
//	{ "type": "...", "todoId": "...", "title": "...",
//	  "completed": false, "timestamp": "RFC3339" }
type TodoEvent struct {
	Type      string    `json:"type"`
	TodoID    uuid.UUID `json:"todoId"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	Timestamp time.Time `json:"timestamp"`
}

// Publisher publishes domain events. Publishing is best-effort / fire-and-forget
// from the caller's perspective — a failure must never break the HTTP request.
type Publisher interface {
	Publish(ctx context.Context, e TodoEvent)
	Close() error
}

// NoopPublisher is used when Kafka is not configured. It does nothing, so the
// app runs identically with or without Kafka.
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, TodoEvent) {}
func (NoopPublisher) Close() error                       { return nil }
