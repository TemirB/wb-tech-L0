package service

import "time"

type LookupSource string

const (
	SourceCache LookupSource = "cache"
	SourceDB    LookupSource = "db"
)

type LookupStats struct {
	Source  LookupSource
	CacheMs float64
	DBMs    float64
}

type UpsertStats struct {
	DBWriteMs float64
}

func convertToMs(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}
