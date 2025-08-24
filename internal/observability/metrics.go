package observability

type Metrics interface {
	ObserveLookup(source string, cacheMs, dbMs float64)
	ObserveUpsert(dbWriteMs float64)
	ObserveHTTP(method, route string, status int, durMs float64)
	ObserveKafka(processMs float64, ok bool)
	IncCacheHit()
	IncCacheMiss()
}

// type Noop struct{}

// func (Noop) ObserveLookup(string, float64, float64)   {}
// func (Noop) ObserveUpsert(float64)                    {}
// func (Noop) ObserveHTTP(string, string, int, float64) {}
// func (Noop) ObserveKafka(float64, bool)               {}
// func (Noop) IncCacheHit()                             {}
// func (Noop) IncCacheMiss()                            {}
