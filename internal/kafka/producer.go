package kafka

import (
	"context"
	"encoding/json"
	"time"

	"async-pix-settlement-api/internal/transaction"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
			WriteTimeout: 10 * time.Second,
		},
	}
}

func (p *Producer) PublishTransfer(ctx context.Context, event transaction.TransferEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.TransactionID.String()),
		Value: body,
		Time:  time.Now(),
	})
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
