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

func (l *LRU) Get(uid string) (domain.Order, bool) {
	return l.c.Get(uid)
}

func (l *LRU) Set(uid string, o domain.Order) {
	l.c.Add(uid, o)
}
