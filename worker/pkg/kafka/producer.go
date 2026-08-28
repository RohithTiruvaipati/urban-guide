package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// ResultProducer publishes chunk render completion results to Kafka.
type ResultProducer struct {
	writer *kafka.Writer
	topic  string
}

// NewResultProducer initializes a Kafka writer for the results topic.
func NewResultProducer(brokers []string, topic string) *ResultProducer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
	}
	return &ResultProducer{
		writer: writer,
		topic:  topic,
	}
}

// PublishResult serializes and publishes a ChunkResultMsg to the results topic.
func (p *ResultProducer) PublishResult(ctx context.Context, result ChunkResultMsg) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result message: %w", err)
	}

	key := fmt.Sprintf("%s-%d", result.JobID, result.ChunkIndex)

	msg := kafka.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("failed to write result message to kafka: %w", err)
	}

	return nil
}

// Close gracefully closes the underlying Kafka writer.
func (p *ResultProducer) Close() error {
	return p.writer.Close()
}
