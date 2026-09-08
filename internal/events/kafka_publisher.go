package events

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaPublisher writes TodoEvents to Kafka. Publishing is fire-and-forget:
// events are sent asynchronously and failures are logged, never surfaced to the
// HTTP caller — the CRUD write path stays synchronous and unaffected.
type KafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher creates a publisher writing to the todo-events topic on the
// given brokers (comma-separated host:port list).
func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        Topic,
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 50 * time.Millisecond,
			Async:        true, // fire-and-forget; do not block the request
			// Log delivery errors instead of failing anything.
			Completion: func(messages []kafka.Message, err error) {
				if err != nil {
					log.Printf("kafka publish error: %v", err)
				}
			},
		},
	}
}

func (p *KafkaPublisher) Publish(ctx context.Context, e TodoEvent) {
	payload, err := json.Marshal(e)
	if err != nil {
		log.Printf("kafka marshal error: %v", err)
		return
	}
	// Key by todo id so events for the same todo land on the same partition
	// (preserves per-todo ordering).
	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(e.TodoID.String()),
		Value: payload,
	})
	if err != nil {
		log.Printf("kafka write error: %v", err)
	}
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}
