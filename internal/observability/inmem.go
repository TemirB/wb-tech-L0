package observability

import "sync"

type Inmem struct {
	mu     sync.Mutex
	last   []any
	max    int
	totals struct {
		cacheHits, cacheMiss int
	}
}

func NewInmem(max int) *Inmem {
	return &Inmem{
		max: max,
	}
}

func (m *Inmem) push(v any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = append(m.last, v)
	if len(m.last) > m.max {
		m.last = m.last[1:]
	}
}

func (m *Inmem) ObserveLookup(source string, cacheMs, dbMs float64) {
	m.push(struct {
		Kind          string
		Source        string
		CacheMs, DbMs float64
	}{"lookup", source, cacheMs, dbMs})
}

func (m *Inmem) ObserveUpsert(dbWriteMs float64) {
	m.push(struct {
		Kind      string
		DbWriteMs float64
	}{"upsert", dbWriteMs})
}

func (m *Inmem) ObserveHTTP(method, route string, status int, durMs float64) {
	m.push(struct {
		Kind          string
		Method, Route string
		Status        int
		Dur           float64
	}{"http", method, route, status, durMs})
}

func (m *Inmem) ObserveKafka(processMs float64, ok bool) {
	m.push(struct {
		Kind string
		Dur  float64
		OK   bool
	}{"kafka", processMs, ok})
}

func (m *Inmem) IncCacheHit() {
	m.mu.Lock()
	m.totals.cacheHits++
	m.mu.Unlock()
}
func (m *Inmem) IncCacheMiss() {
	m.mu.Lock()
	m.totals.cacheMiss++
	m.mu.Unlock()
}
