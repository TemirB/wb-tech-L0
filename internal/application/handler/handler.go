package handler

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/TemirB/wb-tech-L0/internal/config"
	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/pkg/breaker"
	"github.com/TemirB/wb-tech-L0/internal/pkg/retry"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

var ErrKafkaFetch = errors.New("kafka fetch error")
var ErrBadJSON = errors.New("bad json")
var ErrUpsert = errors.New("upsert failed")
var ErrCommit = errors.New("errorToCommit")

type Service interface {
	Upsert(ctx context.Context, order *domain.Order) error
}

type Handler struct {
	service Service
	reader  *kafka.Reader
	breaker *breaker.Breaker
	logger  *zap.Logger
}

func NewHandler(service Service, reader *kafka.Reader, breaker *breaker.Breaker, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		reader:  reader,
		breaker: breaker,
		logger:  logger,
	}
}

func (h *Handler) HandleUpsert(ctx context.Context, message kafka.Message, retryPolicy config.Retry) error {
	// if err := h.breaker.Allow(); err != nil {
	// 	h.logger.Warn(
	// 		"circuit breaker is open",
	// 		zap.Error(err),
	// 	)
	// 	time.Sleep(100 * time.Millisecond)
	// 	return fmt.Errorf("circuit breaker open: %w", err)
	// }

	var order domain.Order
	if err := json.Unmarshal(message.Value, &order); err != nil {
		h.logger.Error("bad json format",
			zap.Error(err),
			zap.ByteString("raw_message", message.Value),
		)

		return ErrBadJSON
	}

	processingErr := retry.Do(ctx, retryPolicy,
		func() error { return h.service.Upsert(ctx, &order) },
	)

	if processingErr != nil {
		h.logger.Error("upsert failed after retries",
			zap.String("order_uid", order.OrderUID),
			zap.Error(processingErr),
		)
		// h.breaker.Failure()

		return ErrUpsert
	}

	// h.breaker.Success()
	h.logger.Info("successfully processed order",
		zap.String("order_uid", order.OrderUID),
	)

	// if err := h.reader.CommitMessages(ctx, message); err != nil {
	// 	h.logger.Error("commit failed",
	// 		zap.String("order_uid", order.OrderUID),
	// 		zap.Error(err),
	// 	)
	// 	return fmt.Errorf("commit failed: %w", err)
	// }

	return nil
}
