package kafka

import (
	"context"
	"errors"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/TemirB/wb-tech-L0/internal/config"
)

//go:generate mockgen -destination=../mocks/mock_order_store.go -package=mocks github.com/TemirB/wb-tech-L0/internal/kafka OrderStore

type UpsertHandler interface {
	HandleUpsert(ctx context.Context, message kafka.Message, retryPolicy config.Retry) error
}

type Handler struct {
	upsertHandler UpsertHandler
	reader        *kafka.Reader
	logger        *zap.Logger
}

func New(upsertHandler UpsertHandler, reader *kafka.Reader, logger *zap.Logger) *Handler {
	return &Handler{
		upsertHandler: upsertHandler,
		reader:        reader,
		logger:        logger,
	}
}

func (h *Handler) Start(ctx context.Context, retryPolicy config.Retry) {
	h.logger.Info("Starting Kafka consumer",
		zap.Strings("brokers", h.reader.Config().Brokers),
		zap.String("topic", h.reader.Config().Topic),
		zap.String("group", h.reader.Config().GroupID),
		zap.Strings("group_topic", h.reader.Config().GroupTopics),
	)

	go func() {
		defer h.reader.Close()
		for {
			select {
			case <-ctx.Done():
				h.logger.Info("Stopping Kafka consumer due to context cancellation")
				return
			default:
				h.logger.Debug("Attempting to fetch message from Kafka")
				m, err := h.reader.FetchMessage(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					h.logger.Warn("Failed to fetch message", zap.Error(err))
					// time.Sleep(time.Second)
					continue
				}

				h.logger.Info(
					"Message accepted",
					zap.String("Topic", m.Topic),
					zap.Int("partition", m.Partition),
					zap.Int64("offset", m.Offset),
				)

				// Передаем сообщение в обработчик
				err = h.upsertHandler.HandleUpsert(ctx, m, retryPolicy)
				if err != nil {
					h.logger.Warn("Handler failed", zap.Error(err))
				}
				err = h.reader.CommitMessages(ctx, m)
				if err != nil {
					h.logger.Error(
						"Message commiting error",
						zap.Error(err),
					)
				}
			}
		}
	}()
}
