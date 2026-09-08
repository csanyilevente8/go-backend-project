package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"

	"github.com/csanyilevente8/go-backend-project/internal/repository"
)

// Consumer reads TodoEvents from Kafka (consumer group "activity-logger") and
// writes each to the activity_log table. Runs as a background goroutine; it is
// independent of the HTTP request path.
type Consumer struct {
	reader   *kafka.Reader
	activity *repository.ActivityRepository
}

func NewConsumer(brokers []string, activity *repository.ActivityRepository) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   Topic,
			GroupID: "activity-logger",
		}),
		activity: activity,
	}
}

// Run consumes until the context is cancelled. At-least-once delivery: the
// offset is committed after a successful DB write.
func (c *Consumer) Run(ctx context.Context) {
	log.Println("activity consumer started")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("activity consumer stopping")
				return
			}
			log.Printf("consumer read error: %v", err)
			continue
		}

		var e TodoEvent
		if err := json.Unmarshal(msg.Value, &e); err != nil {
			log.Printf("consumer: skipping unparseable message: %v", err)
			continue
		}

		detail := describe(e)
		if err := c.activity.Insert(ctx, e.TodoID, e.Type, &detail); err != nil {
			log.Printf("consumer: failed to insert activity: %v", err)
			// Do not advance past a failed write on a fatal ctx; loop continues.
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

// describe builds a short human-readable detail string for the activity entry.
func describe(e TodoEvent) string {
	switch e.Type {
	case TypeCreated:
		return "Created: " + e.Title
	case TypeUpdated:
		return "Updated: " + e.Title
	case TypeCompleted:
		if e.Completed {
			return "Marked complete: " + e.Title
		}
		return "Marked incomplete: " + e.Title
	case TypeDeleted:
		return "Deleted: " + e.Title
	default:
		return e.Title
	}
}
