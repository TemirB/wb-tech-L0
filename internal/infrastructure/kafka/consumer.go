package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

type Handler func(ctx context.Context, payload []byte) error

type Consumer struct {
	reader  *kafka.Reader
	handler Handler
}

func NewConsumer(brokers []string, topic, group string, handler Handler) *Consumer {
	reader := kafka.NewReader(
		kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     group,
			StartOffset: kafka.LastOffset,
			MaxWait:     500 * time.Millisecond,
		},
	)
	return &Consumer{
		reader:  reader,
		handler: handler,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.reader.Close()

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		if err := c.handler(ctx, msg.Value); err != nil {
			// ретраи DLQ
			continue
		}
	}
}
