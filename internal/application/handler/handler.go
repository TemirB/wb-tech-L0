package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TemirB/wb-tech-L0/internal/config"
	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/pkg/breaker"
	"github.com/TemirB/wb-tech-L0/internal/pkg/retry"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

var (
	ErrBadJSON     = errors.New("bad json")
	ErrUpsert      = errors.New("upsert failed")
	ErrCircuitOpen = errors.New("circuit breaker open")
)

type Service interface {
	Upsert(ctx context.Context, order *domain.Order) error
}

type Handler struct {
	service     Service
	breaker     *breaker.Breaker
	logger      *zap.Logger
	retryPolicy config.Retry
}

func NewHandler(service Service, brk *breaker.Breaker, retryPolicy config.Retry, logger *zap.Logger) *Handler {
	return &Handler{
		service:     service,
		breaker:     brk,
		logger:      logger,
		retryPolicy: retryPolicy,
	}
}

// Handle — called by the consumer to process a single message.
// The consumer commits the offset itself after successfully returning nil.
func (h *Handler) Handle(ctx context.Context, message kafkago.Message) error {
	if err := h.breaker.Allow(); err != nil {
		h.logger.Warn("circuit breaker is open",
			zap.Error(err),
			zap.Int("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)
		return fmt.Errorf("%w: %v", ErrCircuitOpen, err)
	}

	var order domain.Order
	if err := json.Unmarshal(message.Value, &order); err != nil {
		h.logger.Error("bad json format",
			zap.Error(err),
			zap.Int("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)
		h.breaker.Failure()
		return ErrBadJSON
	}
	if order.OrderUID == "" {
		h.logger.Error("missing order_uid",
			zap.Int("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)
		h.breaker.Failure()
		return ErrBadJSON
	}

	if err := retry.Do(ctx, h.retryPolicy, func() error {
		return h.service.Upsert(ctx, &order)
	}); err != nil {
		h.logger.Error("upsert failed after retries",
			zap.String("order_uid", order.OrderUID),
			zap.Error(err),
			zap.Int("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)
		h.breaker.Failure()
		return ErrUpsert
	}

	h.breaker.Success()
	h.logger.Info("successfully processed order",
		zap.String("order_uid", order.OrderUID),
		zap.Int("partition", message.Partition),
		zap.Int64("offset", message.Offset),
		zap.Int("key_bytes", len(message.Key)),
		zap.Int("value_bytes", len(message.Value)),
	)
	return nil
}
