package service

import (
	"context"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/observability"
)

func TestUpsert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	l := zap.NewNop()
	m := observability.NewNoop()
	order := &domain.Order{
		OrderUID: "123",
	}

	testCases := []struct {
		name string

		setupMocks func() *Service
		wantErr    error
	}{
		{
			name: "Success",

			setupMocks: func() *Service {
				storage := NewMockStorage(ctrl)
				cache := NewMockCache(ctrl)

				storage.EXPECT().Upsert(ctx, order).Return(nil)
				cache.EXPECT().Set(order)
				return NewService(cache, storage, l, m)
			},
		},
		{
			name: "DB error",

			setupMocks: func() *Service {
				storage := NewMockStorage(ctrl)
				storage.EXPECT().Upsert(ctx, order).Return(pgx.ErrNoRows)
				return NewService(nil, storage, l, m)
			},

			wantErr: pgx.ErrNoRows,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupMocks()
			err := s.Upsert(ctx, order)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetByUID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()

	testUID := "88"
	order := &domain.Order{
		OrderUID: testUID,
	}

	l := zap.NewNop()
	m := observability.NewNoop()

	testCases := []struct {
		name string

		setupMocks func() *Service

		expected *domain.Order
		wantErr  error
	}{
		{
			name: "Order fetched from cache",

			setupMocks: func() *Service {
				cache := NewMockCache(ctrl)

				cache.EXPECT().Get(testUID).Return(order, true)

				return NewService(cache, nil, l, m)
			},

			expected: order,
		},
		{
			name: "Order fetched from DB",

			setupMocks: func() *Service {
				cache := NewMockCache(ctrl)
				storage := NewMockStorage(ctrl)

				cache.EXPECT().Get(testUID).Return(nil, false)
				storage.EXPECT().GetByUID(ctx, testUID).Return(order, nil)
				cache.EXPECT().Set(order)

				return NewService(cache, storage, l, m)
			},

			expected: order,
		},
		{
			name: "Cant find order",

			setupMocks: func() *Service {
				cache := NewMockCache(ctrl)
				storage := NewMockStorage(ctrl)

				cache.EXPECT().Get(testUID).Return(nil, false)
				storage.EXPECT().GetByUID(ctx, testUID).Return(nil, pgx.ErrNoRows)

				return NewService(cache, storage, l, m)
			},

			wantErr: pgx.ErrNoRows,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.setupMocks()
			order, err := s.GetByUID(ctx, testUID)

			if tc.wantErr != nil {
				require.Error(t, err)
				require.Nil(t, order)
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expected, order)
			}
		})
	}
}

// func TestService_GetByUIDWithStats_FromCache(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	cache := mocks.NewMockCache(ctrl)
// 	storage := mocks.NewMockStorage(ctrl)
// 	metrics := mocks.NewMockMetrics(ctrl)
// 	logger := zap.NewNop()

// 	order := &domain.Order{OrderUID: "xyz"}
// 	cache.EXPECT().Get("xyz").Return(order, true)
// 	metrics.EXPECT().IncCacheHit()
// 	metrics.EXPECT().ObserveLookup("cache", gomock.Any(), float64(0))

// 	svc := appsvc.NewService(cache, storage, logger, metrics)
// 	got, st, err := svc.GetByUIDWithStats(context.Background(), "xyz")
// 	if err != nil {
// 		t.Fatalf("unexpected error: %v", err)
// 	}
// 	if got != order || st.Source != appsvc.SourceCache {
// 		t.Fatalf("unexpected result: %#v, %#v", got, st)
// 	}
// }

// func TestService_GetByUIDWithStats_FromDB(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	cache := mocks.NewMockCache(ctrl)
// 	storage := mocks.NewMockStorage(ctrl)
// 	metrics := mocks.NewMockMetrics(ctrl)
// 	logger := zap.NewNop()

// 	order := &domain.Order{OrderUID: "abc"}
// 	cache.EXPECT().Get("abc").Return((*domain.Order)(nil), false)
// 	metrics.EXPECT().IncCacheMiss()
// 	storage.EXPECT().GetByUID(gomock.Any(), "abc").Return(order, nil)
// 	cache.EXPECT().Set(order)
// 	metrics.EXPECT().ObserveLookup("db", gomock.Any(), gomock.Any())

// 	svc := appsvc.NewService(cache, storage, logger, metrics)
// 	got, st, err := svc.GetByUIDWithStats(context.Background(), "abc")
// 	if err != nil {
// 		t.Fatalf("unexpected error: %v", err)
// 	}
// 	if got != order || st.Source != appsvc.SourceDB {
// 		t.Fatalf("unexpected result: %#v, %#v", got, st)
// 	}
// }

// func TestService_GetByUIDWithStats_DBError(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	cache := mocks.NewMockCache(ctrl)
// 	storage := mocks.NewMockStorage(ctrl)
// 	metrics := mocks.NewMockMetrics(ctrl)
// 	logger := zap.NewNop()

// 	cache.EXPECT().Get("nope").Return((*domain.Order)(nil), false)
// 	metrics.EXPECT().IncCacheMiss()
// 	storage.EXPECT().GetByUID(gomock.Any(), "nope").Return(nil, domain.ErrNotFound)

// 	svc := appsvc.NewService(cache, storage, logger, metrics)
// 	_, _, err := svc.GetByUIDWithStats(context.Background(), "nope")
// 	if err == nil {
// 		t.Fatalf("expected error, got nil")
// 	}
// }
