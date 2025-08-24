package cache

import (
	"context"

	"github.com/TemirB/wb-tech-L0/internal/config"
	"github.com/TemirB/wb-tech-L0/internal/domain"

	lru "github.com/hashicorp/golang-lru/v2"
)

//go:generate mockgen -source internal/cache/cache.go -destination=internal/mocks/cache_mock_test.go -package=mocks

type repo interface {
	GetByUID(ctx context.Context, uid string) (*domain.Order, error)
	RecentOrderIDs(ctx context.Context, limit int) ([]string, error)
}

type Cache struct {
	lru *lru.Cache[string, domain.Order]
}

func New(size int) (*Cache, error) {
	c, err := lru.New[string, domain.Order](size)
	if err != nil {
		return nil, err
	}
	return &Cache{c}, nil
}

func (c *Cache) Warm(ctx context.Context, cfg config.Config, repo repo) {
	if ids, err := repo.RecentOrderIDs(ctx, cfg.CacheCap); err == nil {
		for _, id := range ids {
			if o, err := repo.GetByUID(ctx, id); err == nil {
				c.Set(o)
			}
		}
	}
}

func (c *Cache) Get(uid string) (*domain.Order, bool) {
	order, ok := c.lru.Get(uid)
	return &order, ok
}

func (c *Cache) Set(order *domain.Order) {
	c.lru.Add(order.OrderUID, *order)
}
