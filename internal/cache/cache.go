package cache

import (
	"github.com/TemirB/wb-tech-L0/internal/domain"

	lru "github.com/hashicorp/golang-lru/v2"
)

type LRU struct {
	c *lru.Cache[string, domain.Order]
}

func New(size int) (*LRU, error) {
	c, err := lru.New[string, domain.Order](size)
	if err != nil {
		return nil, err
	}
	return &LRU{c}, nil
}

func (l *LRU) Get(uid string) (*domain.Order, bool) {
	order, ok := l.c.Get(uid)
	return &order, ok
}

func (l *LRU) Set(order *domain.Order) {
	l.c.Add(order.OrderUID, *order)
}

func (c *LRU) Warm(list []*domain.Order) {
	for _, o := range list {
		c.Set(o)
	}
}
