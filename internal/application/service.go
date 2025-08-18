package application

import (
	"context"

	"github.com/TemirB/wb-tech-L0/internal/domain"
	"go.uber.org/zap"
)

type Cache interface {
	Set(string, domain.Order) error
	Get(string) (domain.Order, error)
}

type Storage interface {
	Upsert(domain.Order) error
	GetByUID(string) (domain.Order, error)
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

func (s *Service) Upsert(ctx context.Context, order domain.Order) error {
	if err := s.storage.Upsert(order); err != nil {
		s.logger.Error(
			"Error while upserting order in table",
			zap.Error(err),
		)
		return err
	}

	if err := s.cache.Set(order.OrderUID, order); err != nil {
		s.logger.Error(
			"Error while set order in cache",
			zap.String("order_uid", order.OrderUID),
			zap.Error(err),
		)
	}
	return nil
}

func (s *Service) GetByUID(ctx context.Context, uid string) (*domain.Order, error) {
	order, err := s.cache.Get(uid)
	if err == nil {
		return &order, nil
	}

	order, err = s.storage.GetByUID(uid)
	if err != nil {
		s.logger.Error(
			"Can't find order",
			zap.String("order_uid", uid),
			zap.Error(err),
		)
		return nil, err
	}

	if err := s.cache.Set(uid, order); err != nil {
		s.logger.Warn(
			"Error while set order in cache",
			zap.String("order_uid", uid),
			zap.Error(err),
		)
	}

	return &order, nil
}
