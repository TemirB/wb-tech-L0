package command

import (
	"context"

	"github.com/TemirB/WB-TECH-L0/internal/domain"
	"github.com/TemirB/WB-TECH-L0/internal/pkg/circuit"
)

type UpsertOrderHandler struct {
	repo    domain.OrderRepository
	cache   domain.Cache
	breaker *circuit.Breaker
}

func NewUpsertOrderHandler(repo domain.OrderRepository, cache domain.Cache) *UpsertOrderHandler {
	return &UpsertOrderHandler{
		repo:  repo,
		cache: cache,
	}
}

func (h *UpsertOrderHandler) Handle(ctx context.Context, order domain.Order) error {
	if err := h.repo.Upsert(ctx, &order); err != nil {
		return err
	}
	h.cache.Set(order.OrderUID, order)
	return nil
}
