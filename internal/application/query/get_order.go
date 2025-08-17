package query

import (
	"context"

	"github.com/TemirB/WB-TECH-L0/internal/domain"
)

type GetOrderHandler struct {
	repo  domain.OrderRepository
	cache domain.Cache
}

func NewGetOrderHandler(repo domain.OrderRepository, cache domain.Cache) *GetOrderHandler {
	return &GetOrderHandler{
		repo:  repo,
		cache: cache,
	}
}

func (h *GetOrderHandler) Handle(ctx context.Context, UID string) (*domain.Order, error) {
	if v, ok := h.cache.Get(UID); ok {
		return &v, nil
	}
	order, err := h.repo.GetByID(ctx, UID)
	if err != nil {
		return nil, err
	}
	h.cache.Set(UID, *order)
	return order, nil
}
