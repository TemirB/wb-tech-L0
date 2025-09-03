package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TemirB/wb-tech-L0/internal/application/service"
	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/observability"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestServer_GetOrder(t *testing.T) {
	type serviceResponse struct {
		order  *domain.Order
		stats  service.LookupStats
		err    error
		errMsg string
	}

	tests := []struct {
		name           string
		path           string
		serviceResp    serviceResponse
		expectedStatus int
		expectedBody   string
		checkHeaders   func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "successful get order",
			path: "/order/test-uid",
			serviceResp: serviceResponse{
				order: &domain.Order{
					OrderUID: "test-uid",
				},
				stats: service.LookupStats{
					CacheMs: 10,
					DBMs:    20,
					Source:  service.SourceCache,
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"order_uid": "test-uid"`,
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				require.Equal(t, "cache", w.Header().Get("X-Source"))
				require.Equal(t, "10.00", w.Header().Get("X-Cache-Time"))
				require.Equal(t, "20.00", w.Header().Get("X-DB-Time"))
			},
		},
		{
			name:           "missing order id",
			path:           "/order/",
			serviceResp:    serviceResponse{},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "order id required",
		},
		{
			name: "order not found",
			path: "/order/non-existent",
			serviceResp: serviceResponse{
				err:    errors.New("not found"),
				errMsg: "no order with this id",
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "no order with this id",
		},
		{
			name: "service error",
			path: "/order/error-uid",
			serviceResp: serviceResponse{
				err:    errors.New("internal error"),
				errMsg: "no order with this id",
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "no order with this id",
		},
		{
			name: "successful get from db",
			path: "/order/db-uid",
			serviceResp: serviceResponse{
				order: &domain.Order{
					OrderUID: "db-uid",
				},
				stats: service.LookupStats{
					CacheMs: 0,
					DBMs:    30,
					Source:  service.SourceDB,
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"order_uid": "db-uid"`,
			checkHeaders: func(t *testing.T, w *httptest.ResponseRecorder) {
				require.Equal(t, "db", w.Header().Get("X-Source"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := NewMockServerWithStats(ctrl)
			logger := zaptest.NewLogger(t)
			metrics := observability.NewNoop()

			server := New(mockService, logger, metrics)

			if strings.TrimPrefix(tt.path, "/order/") != "" && tt.expectedStatus != http.StatusBadRequest {
				uid := strings.TrimPrefix(tt.path, "/order/")
				mockService.EXPECT().
					GetByUIDWithStats(gomock.Any(), uid).
					Return(tt.serviceResp.order, tt.serviceResp.stats, tt.serviceResp.err)
			}

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			server.Handler().ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			require.Contains(t, w.Body.String(), tt.expectedBody)

			if tt.checkHeaders != nil {
				tt.checkHeaders(t, w)
			}
		})
	}
}

func TestServer_UpsertOrder(t *testing.T) {
	type request struct {
		body        string
		contentType string
	}

	type serviceResponse struct {
		stats service.UpsertStats
		err   error
	}

	tests := []struct {
		name           string
		request        request
		serviceResp    serviceResponse
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "successful upsert",
			request: request{
				contentType: "application/json",
				body: `{
					"order_uid": "test-uid"
				}`,
			},
			serviceResp: serviceResponse{
				stats: service.UpsertStats{
					DBWriteMs: 15,
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"order_uid": "test-uid"`,
		},
		{
			name: "invalid content type",
			request: request{
				contentType: "text/plain",
				body:        `{"order_uid": "test-uid"}`,
			},
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedBody:   "Content-Type must be application/json",
		},
		{
			name: "invalid json",
			request: request{
				contentType: "application/json",
				body:        `{"order_uid": "test-uid"`,
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "bad json",
		},
		{
			name: "missing order_uid",
			request: request{
				contentType: "application/json",
				body:        `{"track_number": "WBILMTESTTRACK"}`,
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "order_uid is required",
		},
		{
			name: "service error",
			request: request{
				contentType: "application/json",
				body: `{
					"order_uid": "error-uid",
					"track_number": "WBILMTESTTRACK"
				}`,
			},
			serviceResp: serviceResponse{
				err: errors.New("service error"),
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Service error",
		},
		{
			name: "unknown fields in json",
			request: request{
				contentType: "application/json",
				body: `{
					"order_uid": "test-uid",
					"unknown_field": "value"
				}`,
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "bad json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockService := NewMockServerWithStats(ctrl)
			logger := zap.NewNop()
			metrics := observability.NewNoop()

			server := New(mockService, logger, metrics)

			// Set up service mock expectations for valid requests
			if tt.request.contentType == "application/json" &&
				tt.expectedStatus != http.StatusBadRequest &&
				tt.expectedStatus != http.StatusUnsupportedMediaType &&
				strings.Contains(tt.request.body, `"order_uid"`) {
				var order domain.Order
				if err := json.Unmarshal([]byte(tt.request.body), &order); err == nil && order.OrderUID != "" {
					mockService.EXPECT().
						UpsertWithStats(gomock.Any(), gomock.Any()).
						Return(tt.serviceResp.stats, tt.serviceResp.err)
				}
			}

			req := httptest.NewRequest("POST", "/order/", bytes.NewReader([]byte(tt.request.body)))
			req.Header.Set("Content-Type", tt.request.contentType)
			w := httptest.NewRecorder()

			server.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" && !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain '%s', got '%s'", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestServer_ListenAndServe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := NewMockServerWithStats(ctrl)
	logger := zaptest.NewLogger(t)
	metrics := observability.NewNoop()

	server := New(mockService, logger, metrics)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test that server starts and shuts down properly
	err := server.ListenAndServe(ctx, ":0") // Use port 0 for random available port
	if err != nil && err != http.ErrServerClosed {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateOrder(t *testing.T) {
	tests := []struct {
		name    string
		order   domain.Order
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid order",
			order: domain.Order{
				OrderUID: "test-uid",
			},
			wantErr: false,
		},
		{
			name:    "missing order_uid",
			order:   domain.Order{},
			wantErr: true,
			errMsg:  "order_uid is required",
		},
		{
			name: "empty order_uid",
			order: domain.Order{
				OrderUID: "",
			},
			wantErr: true,
			errMsg:  "order_uid is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOrder(tt.order)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if err.Error() != tt.errMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple struct",
			input:    map[string]string{"key": "value"},
			expected: `{"key": "value"}`,
		},
		{
			name:     "empty struct",
			input:    struct{}{},
			expected: `{}`,
		},
		{
			name: "order struct",
			input: domain.Order{
				OrderUID:    "test-uid",
				TrackNumber: "WBILMTESTTRACK",
			},
			expected: `"order_uid": "test-uid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			writeJSON(w, tt.input)

			require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			cleanBody := strings.ReplaceAll(w.Body.String(), " ", "")
			cleanBody = strings.ReplaceAll(cleanBody, "\n", "")

			cleanExpected := strings.ReplaceAll(tt.expected, " ", "")
			cleanExpected = strings.ReplaceAll(cleanExpected, "\n", "")

			require.Contains(t, cleanBody, cleanExpected)
		})
	}
}
