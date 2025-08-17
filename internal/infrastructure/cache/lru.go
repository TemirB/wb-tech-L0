package cache

import (
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/TemirB/WB-TECH-L0/internal/domain"
)

type LRU struct {
	cache *lru.Cache[string, domain.Order]
}

func MustNewLRU(size int) *LRU {
	cache, err := lru.New[string, domain.Order](size)
	if err != nil {
		panic(err)
	}
	return &LRU{cache}
}

func (l *LRU) Get(UID string) (domain.Order, bool) {
	return l.cache.Get(UID)
}

func (l *LRU) Set(UID string, order domain.Order) {
	l.cache.Add(UID, order)
}
