package kafka

import (
	"context"
	"log/slog"
	"time"

	"github.com/TemirB/WB-TECH-L0/internal/pkg/pool"

	"github.com/segmentio/kafka-go"
)

type Handler func(context.Context, []byte) error

type Consumer struct {
	reader  *kafka.Reader
	parts   map[int]*pool.Pool
	logger  *slog.Logger
	workers int
	handler Handler
}

func NewConsumer(brokers []string, topic, group string, handler Handler, workers int, logger *slog.Logger) *Consumer {
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
		parts:   map[int]*pool.Pool{},
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
		c.parts[p] = pool.New(c.workers)
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
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		c.ensure(m.Partition)
		pm := c.parts[m.Partition]
		msg := m
		pm.Submit(func() {
			if err := c.handler(ctx, msg.Value); err != nil {
				c.logger.Error("kafka_handler_error", "err", err, "partition", msg.Partition, "offset", msg.Offset)
				return
			}
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.Error("kafka_commit_error", "err", err, "partition", msg.Partition, "offset", msg.Offset)
			}
		})
	}
}
