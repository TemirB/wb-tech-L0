package observability

import "sync"

type Inmem struct {
	mu     sync.Mutex
	last   []*observe
	max    int
	totals struct {
		cacheHits, cacheMiss int
	}
}

type observe struct {
	Kind string

	// Lookup fields
	lSorce          string
	lCacheMs, lDbMs float64
	// Upsert fileds
	uDbWriteMs float64
	// HTTP fields
	hMethod, hRoute string
	hStatus         int
	hDur            float64
	// Kafka fields
	kDur float64
	kOK  bool
}

func NewLookup(source string, cacheMs, dbMs float64) *observe {
	return &observe{
		Kind:     "lookup",
		lSorce:   source,
		lCacheMs: cacheMs,
		lDbMs:    dbMs,
	}
}

func NewUpsert(dbWriteMs float64) *observe {
	return &observe{
		Kind:       "upsert",
		uDbWriteMs: dbWriteMs,
	}
}

func NewHTTP(method, route string, status int, durMs float64) *observe {
	return &observe{
		Kind:    "http",
		hMethod: method,
		hRoute:  route,
		hStatus: status,
		hDur:    durMs,
	}
}

func NewKafka(processMs float64, ok bool) *observe {
	return &observe{
		Kind: "kafka",
		kDur: processMs,
		kOK:  ok,
	}
}

func NewInmem(max int) *Inmem {
	return &Inmem{
		max: max,
	}
}

func (m *Inmem) push(v *observe) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = append(m.last, v)
	if len(m.last) > m.max {
		m.last = m.last[1:]
	}
}

func (m *Inmem) ObserveLookup(source string, cacheMs, dbMs float64) {
	m.push(NewLookup(source, cacheMs, dbMs))
}

func (m *Inmem) ObserveUpsert(dbWriteMs float64) {
	m.push(NewUpsert(dbWriteMs))
}

func (m *Inmem) ObserveHTTP(method, route string, status int, durMs float64) {
	m.push(NewHTTP(method, route, status, durMs))
}

func (m *Inmem) ObserveKafka(processMs float64, ok bool) {
	m.push(NewKafka(processMs, ok))
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
