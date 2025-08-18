package kafka

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/segmentio/kafka-go"
)

type Handler func(context.Context, []byte) error

type Consumer struct {
	reader  *kafka.Reader
	parts   map[int]*Pool
	workers int
	handler Handler
	logger  *zap.Logger
}

func NewConsumer(brokers []string, topic, group string, handler Handler, workers int, logger *zap.Logger) *Consumer {
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
		parts:   make(map[int]*Pool),
		logger:  logger,
		workers: workers,
		handler: handler,
	}
}

func (c *Consumer) ensure(p int) {
	if _, ok := c.parts[p]; !ok {
		if c.workers < 1 {
			c.workers = 1
		}
		c.parts[p] = NewPool(c.workers)
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer func() {
		for _, p := range c.parts {
			p.Close()
			p.Wait()
		}
		_ = c.reader.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			m, err := c.reader.ReadMessage(ctx)
			if err != nil {
				return err
			}

			c.ensure(m.Partition)
			msg := m

			c.parts[m.Partition].Submit(func() {
				if err := c.handler(ctx, msg.Value); err != nil {
					c.logger.Error("Kafka handler error",
						zap.Int("partition", msg.Partition),
						zap.Int64("offset", msg.Offset),
						zap.Error(err),
					)
					return
				}

				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					c.logger.Error("Kafka commit error",
						zap.Int("partition", msg.Partition),
						zap.Int64("offset", msg.Offset),
						zap.Error(err),
					)
				}
			})
		}
	}
}
