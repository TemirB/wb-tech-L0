package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/TemirB/wb-tech-L0/internal/config"
	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/golang/mock/gomock"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	order := domain.Order{
		OrderUID: "some order uid",
	}
	mValue, _ := json.Marshal(order)
	m := kafkago.Message{
		Value: mValue,
	}
	l := zap.NewNop()
	rPolicy := config.Retry{
		Attempts: 1,
	}

	testCases := []struct {
		name string

		badValue   interface{}
		setupMocks func() *Handler
		wantErr    error
	}{
		{
			name: "Success",

			setupMocks: func() *Handler {
				service := NewMockService(ctrl)
				brk := NewMockbrk(ctrl)

				brk.EXPECT().Allow().Return(nil)
				service.EXPECT().Upsert(ctx, &order).Return(nil)
				brk.EXPECT().Success()

				return NewHandler(service, brk, rPolicy, l)
			},
		},
		{
			name: "Circuit breaker is open",

			setupMocks: func() *Handler {
				brk := NewMockbrk(ctrl)

				brk.EXPECT().Allow().Return(errors.New("open"))

				return NewHandler(nil, brk, rPolicy, l)
			},

			wantErr: errors.New("open"),
		},
		{
			name: "missing order_uid",

			badValue: &domain.Order{},
			setupMocks: func() *Handler {
				brk := NewMockbrk(ctrl)

				brk.EXPECT().Allow().Return(nil)
				brk.EXPECT().Failure()
				return NewHandler(nil, brk, rPolicy, l)
			},

			wantErr: ErrBadJSON,
		},
		{
			name: "upsert failed after retries",

			setupMocks: func() *Handler {
				brk := NewMockbrk(ctrl)
				service := NewMockService(ctrl)

				brk.EXPECT().Allow().Return(nil)
				service.EXPECT().Upsert(ctx, &order).Return(errors.New("upsert err"))
				brk.EXPECT().Failure()

				return NewHandler(service, brk, rPolicy, l)
			},

			wantErr: ErrUpsert,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.setupMocks()
			var err error

			if tc.badValue == nil {
				err = h.Handle(ctx, m)
			} else {
				msgValue, _ := json.Marshal(tc.badValue)
				msg := kafkago.Message{
					Value: msgValue,
				}
				err = h.Handle(ctx, msg)
			}

			if tc.wantErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
