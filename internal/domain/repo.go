package domain

import (
	"context"
)

type OrderRepository interface {
	Upsert(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, orderUID string) (*Order, error)
	RecentOrderIDs(ctx context.Context, limit int) ([]string, error)
}

type Cache interface {
	Get(UID string) (Order, bool)
	Set(UID string, order Order)
}
