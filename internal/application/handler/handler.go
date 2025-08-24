package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TemirB/wb-tech-L0/internal/config"
	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/pkg/retry"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

//go:generate mockgen -source internal/application/handler/handler.go -destination=internal/mocks/handler_mock_test.go -package=mocks

var (
	ErrBadJSON     = errors.New("bad json")
	ErrUpsert      = errors.New("upsert failed")
	ErrCircuitOpen = errors.New("circuit breaker open")
)

type Service interface {
	Upsert(ctx context.Context, order *domain.Order) error
}

type brk interface {
	Allow() error
	Failure()
	Success()
}

type Handler struct {
	service     Service
	brk         brk
	logger      *zap.Logger
	retryPolicy config.Retry
}

func NewHandler(service Service, brk brk, retryPolicy config.Retry, logger *zap.Logger) *Handler {
	return &Handler{
		service:     service,
		brk:         brk,
		logger:      logger,
		retryPolicy: retryPolicy,
	}
}

// Handle — called by the consumer to process a single message.
// The consumer commits the offset itself after successfully returning nil.
func (h *Handler) Handle(ctx context.Context, message kafkago.Message) error {
	if err := h.brk.Allow(); err != nil {
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
		h.brk.Failure()
		return ErrBadJSON
	}
	if order.OrderUID == "" {
		h.logger.Error("missing order_uid",
			zap.Int("partition", message.Partition),
			zap.Int64("offset", message.Offset),
		)
		h.brk.Failure()
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
		h.brk.Failure()
		return ErrUpsert
	}

	h.brk.Success()
	h.logger.Info("successfully processed order",
		zap.String("order_uid", order.OrderUID),
		zap.Int("partition", message.Partition),
		zap.Int64("offset", message.Offset),
		zap.Int("key_bytes", len(message.Key)),
		zap.Int("value_bytes", len(message.Value)),
	)
	return nil
}
