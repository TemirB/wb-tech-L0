package service

import (
	"context"
	"time"

	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/observability"
	"go.uber.org/zap"
)

type Cache interface {
	Set(*domain.Order)
	Get(string) (*domain.Order, bool)
}

type Storage interface {
	Upsert(context.Context, *domain.Order) error
	GetByUID(context.Context, string) (*domain.Order, error)
}

type Service struct {
	cache   Cache
	storage Storage
	logger  *zap.Logger
	metrics observability.Metrics
}

func NewService(cache Cache, storage Storage, logger *zap.Logger, metrics observability.Metrics) *Service {
	return &Service{
		cache:   cache,
		storage: storage,
		logger:  logger,
		metrics: metrics,
	}
}

func (s *Service) UpsertWithStats(ctx context.Context, order *domain.Order) (UpsertStats, error) {
	var st UpsertStats

	t0 := time.Now()
	if err := s.storage.Upsert(ctx, order); err != nil {
		s.logger.Error(
			"Error while upserting order in db",
			zap.Error(err),
		)
		return st, err
	}
	st.DBWriteMs = float64(time.Since(t0).Microseconds()) / 1000.0

	s.cache.Set(order)

	s.metrics.ObserveUpsert(st.DBWriteMs)
	s.logger.Info("Order upserted",
		zap.String("order_uid", order.OrderUID),
		zap.Float64("db_write_ms", st.DBWriteMs),
	)

	return st, nil
}

func (s *Service) Upsert(ctx context.Context, order *domain.Order) error {
	_, err := s.UpsertWithStats(ctx, order)
	return err
}

func (s *Service) GetByUID(ctx context.Context, uid string) (*domain.Order, error) {
	o, _, err := s.GetByUIDWithStats(ctx, uid)
	return o, err
}

func (s *Service) GetByUIDWithStats(ctx context.Context, uid string) (*domain.Order, LookupStats, error) {
	var st LookupStats

	// Try cache
	tCacheStart := time.Now()
	if order, ok := s.cache.Get(uid); ok {
		st.Source = SourceCache
		st.CacheMs = convertToMs(tCacheStart)
		s.metrics.IncCacheHit()
		s.metrics.ObserveLookup(string(st.Source), st.CacheMs, 0)

		s.logger.Info("Order fetched from cache",
			zap.String("order_uid", uid),
			zap.Float64("cache_ms", st.CacheMs),
		)

		return order, st, nil
	}

	// Try DB
	s.metrics.IncCacheMiss()
	st.CacheMs = convertToMs(tCacheStart)

	tDbStart := time.Now()
	order, err := s.storage.GetByUID(ctx, uid)
	if err != nil {
		s.logger.Error(
			"Can't find order",
			zap.String("order_uid", uid),
			zap.Error(err),
			zap.Float64("cache_ms", st.CacheMs),
		)
		return nil, st, err
	}

	st.Source = SourceDB
	st.DBMs = convertToMs(tDbStart)

	s.cache.Set(order)

	// metrics
	s.metrics.ObserveLookup(string(st.Source), st.CacheMs, st.DBMs)
	s.logger.Info("Order fetched from DB",
		zap.String("order_uid", uid),
		zap.Float64("cache_ms", st.CacheMs),
		zap.Float64("db_ms", st.DBMs),
	)

	return order, st, nil
}
