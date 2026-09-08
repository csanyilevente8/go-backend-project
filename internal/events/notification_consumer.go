package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"

	"github.com/csanyilevente8/go-backend-project/internal/repository"
)

// NotificationConsumer reads TodoEvents from Kafka in consumer group "notifier"
// and writes a notification row per event. It is a SECOND, independent consumer
// group on the SAME todo-events topic as the activity consumer — this is the
// UC2 fan-out demo: one event lands in both activity_log and notifications,
// each group tracking its own offset. Runs as a background goroutine, off the
// HTTP request path.
type NotificationConsumer struct {
	reader        *kafka.Reader
	notifications *repository.NotificationRepository
}

func NewNotificationConsumer(brokers []string, notifications *repository.NotificationRepository) *NotificationConsumer {
	return &NotificationConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   Topic,
			GroupID: "notifier",
			// Start from the beginning so a fresh group captures existing events
			// (kafka-go otherwise starts at the end for a group with no offset).
			StartOffset: kafka.FirstOffset,
		}),
		notifications: notifications,
	}
}

// Run consumes until the context is cancelled. At-least-once delivery: the
// offset is committed after a successful DB write.
func (c *NotificationConsumer) Run(ctx context.Context) {
	log.Println("notifier consumer started")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("notifier consumer stopping")
				return
			}
			log.Printf("notifier read error: %v", err)
			continue
		}

		var e TodoEvent
		if err := json.Unmarshal(msg.Value, &e); err != nil {
			log.Printf("notifier: skipping unparseable message: %v", err)
			continue
		}

		msgText, ok := notify(e)
		if !ok {
			continue // event type this consumer doesn't notify on
		}
		if err := c.notifications.Insert(ctx, e.TodoID, e.Type, msgText); err != nil {
			log.Printf("notifier: failed to insert notification: %v", err)
		}
	}
}

func (c *NotificationConsumer) Close() error {
	return c.reader.Close()
}

// notify decides whether an event produces a notification and its message. For
// the learning demo we notify on created/completed/deleted (the "interesting"
// changes) and skip plain updates. The bool reports whether to emit.
func notify(e TodoEvent) (string, bool) {
	switch e.Type {
	case TypeCreated:
		return "New todo added: " + e.Title, true
	case TypeCompleted:
		if e.Completed {
			return "Todo completed: " + e.Title, true
		}
		return "Todo reopened: " + e.Title, true
	case TypeDeleted:
		return "Todo deleted: " + e.Title, true
	default:
		return "", false
	}
}
