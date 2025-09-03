package kafka

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type MessageHandler interface {
	Handle(ctx context.Context, msg kafkago.Message) error
}

type Reader interface {
	Config() kafkago.ReaderConfig
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
}

type Consumer struct {
	handler MessageHandler
	reader  Reader
	zlogger *zap.Logger

	workerPoolSize int
	jobs           chan jobItem
}

type jobItem struct {
	msg    kafkago.Message
	result chan error
}

func NewConsumer(handler MessageHandler, reader Reader, logger *zap.Logger) *Consumer {
	workerPoolSize := 4
	if s := os.Getenv("KAFKA_WORKERS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			workerPoolSize = n
		}
	}
	return &Consumer{
		handler:        handler,
		reader:         reader,
		zlogger:        logger,
		workerPoolSize: workerPoolSize,
		jobs:           make(chan jobItem, workerPoolSize*2),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	rc := c.reader.Config()
	c.zlogger.Info("Starting Kafka consumer",
		zap.Strings("brokers", rc.Brokers),
		zap.String("group", rc.GroupID),
		zap.String("topic", rc.Topic),
		zap.Strings("group_topic", rc.GroupTopics),
	)

	for i := 0; i < c.workerPoolSize; i++ {
		go c.worker(ctx, i)
	}

	// Main fetch cycle. Very important: we send the task to the workers and
	// WAIT for the result on the per-message channel. This ensures that the commit
	// offset goes in the order of receiving messages, i.e. without “jumping”.
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.zlogger.Debug("Attempting to fetch message from Kafka")
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				// exit by context
				return
			}
			if isBenignFetchTimeout(err) {
				c.zlogger.Debug("fetch timeout (idle), backing off", zap.Error(err))
				sleepWithContext(ctx, 10*time.Second)
				continue
			}

			// Frequent temporary errors during rebalancing/coordinator = just wait and continue
			c.zlogger.Warn("FetchMessage error, backing off", zap.Error(err))
			sleepWithContext(ctx, 500*time.Millisecond)
			continue
		}

		// We send the message to the workers and wait for it to be completed.
		done := make(chan error, 1)
		select {
		case c.jobs <- jobItem{msg: msg, result: done}:
		case <-ctx.Done():
			return
		}

		var procErr error
		select {
		case procErr = <-done:
		case <-ctx.Done():
			return
		}

		if procErr != nil {
			c.zlogger.Error("handler failed; message will not be committed", zap.Error(procErr),
				zap.String("topic", msg.Topic), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset))
			// A slight delay so as not to twist the problem too aggressively:
			sleepWithContext(ctx, 200*time.Millisecond)
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.zlogger.Warn(
				"commit failed",
				zap.Error(err),
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
			)
			sleepWithContext(ctx, 200*time.Millisecond)
			continue
		}
		c.zlogger.Debug("message committed",
			zap.String("topic", msg.Topic), zap.Int("partition", msg.Partition), zap.Int64("offset", msg.Offset))
	}
}

// worker — message processing worker.
// It receives a specific msg and must send the result to the result channel.
func (c *Consumer) worker(ctx context.Context, id int) {
	logPrefix := fmt.Sprintf("worker-%d", id)
	stdlog := log.New(os.Stdout, logPrefix+" ", log.LstdFlags)

	for {
		select {
		case <-ctx.Done():
			return
		case it := <-c.jobs:
			// Protection against accidental channel closure
			if it.result == nil {
				continue
			}

			msg := it.msg
			start := time.Now()

			err := c.handler.Handle(ctx, msg)

			elapsed := time.Since(start)
			if err != nil {
				c.zlogger.Error("message handling failed",
					zap.Error(err),
					zap.String("topic", msg.Topic),
					zap.Int("partition", msg.Partition),
					zap.Int64("offset", msg.Offset),
					zap.Duration("elapsed", elapsed),
				)
				it.result <- err
				continue
			}

			// Convenient debug trace + short stdout for quick "grazing" under load.
			c.zlogger.Debug("message handled",
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Int("key_bytes", len(msg.Key)),
				zap.Int("value_bytes", len(msg.Value)),
				zap.Duration("elapsed", elapsed),
			)
			stdlog.Printf("ok topic=%s p=%d off=%d bytes=%d", msg.Topic, msg.Partition, msg.Offset, len(msg.Value))

			it.result <- nil
		}
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func isBenignFetchTimeout(err error) bool {
	s := err.Error()
	return strings.Contains(s, "Request Timed Out") ||
		strings.Contains(s, "no messages received from kafka within the allocated time")
}
