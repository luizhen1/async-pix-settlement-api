package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"async-pix-settlement-api/internal/transaction"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        groupID,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: 0,
			StartOffset:    kafka.FirstOffset,
		}),
	}
}

func (c *Consumer) Consume(ctx context.Context, handler func(context.Context, transaction.TransferEvent) error) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		var event transaction.TransferEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			slog.Error("invalid kafka message discarded", "error", err, "topic", msg.Topic, "partition", msg.Partition, "offset", msg.Offset)
			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				return commitErr
			}
			continue
		}

		start := time.Now()
		slog.Info("kafka message received",
			"transaction_id", event.TransactionID,
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)

		if err := handler(ctx, event); err != nil {
			return err
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}

		slog.Info("kafka message committed",
			"transaction_id", event.TransactionID,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
