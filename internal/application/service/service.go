package service

import (
	"context"

	"github.com/TemirB/wb-tech-L0/internal/domain"
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
}

func NewService(cache Cache, storage Storage, logger *zap.Logger) *Service {
	return &Service{
		cache:   cache,
		storage: storage,
		logger:  logger,
	}
}

func (s *Service) Upsert(ctx context.Context, order *domain.Order) error {
	if err := s.storage.Upsert(ctx, order); err != nil {
		s.logger.Error(
			"Error while upserting order in db",
			zap.Error(err),
		)
		return err
	}

	s.cache.Set(order)

	return nil
}

func (s *Service) GetByUID(ctx context.Context, uid string) (*domain.Order, error) {
	order, ok := s.cache.Get(uid)
	if ok {
		return order, nil
	}

	order, err := s.storage.GetByUID(ctx, uid)
	if err != nil {
		s.logger.Error(
			"Can't find order",
			zap.String("order_uid", uid),
			zap.Error(err),
		)
		return nil, err
	}

	s.cache.Set(order)

	return order, nil
}
