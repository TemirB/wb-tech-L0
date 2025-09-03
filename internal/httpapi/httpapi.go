package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/TemirB/wb-tech-L0/internal/application/service"
	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/observability"
	"go.uber.org/zap"
)

//go:generate mockgen -source internal/httpapi/httpapi.go -destination=internal/httpapi/httpapi_mock_test.go -package=httpapi

type ServerWithStats interface {
	GetByUIDWithStats(ctx context.Context, uid string) (*domain.Order, service.LookupStats, error)
	UpsertWithStats(ctx context.Context, order *domain.Order) (service.UpsertStats, error)
}

type Server struct {
	service ServerWithStats
	mux     *http.ServeMux
	logger  *zap.Logger
	metrics observability.Metrics
}

func New(service ServerWithStats, logger *zap.Logger, metrics observability.Metrics) *Server {
	s := &Server{
		service: service,
		logger:  logger,
		mux:     http.NewServeMux(),
		metrics: metrics,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /order/", s.getOrder)
	s.mux.HandleFunc("POST /order/", s.upsertOrder)
	s.mux.Handle("/", http.FileServer(http.Dir(s.staticDir())))
}

func (s *Server) staticDir() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "static")
}

func (s *Server) getOrder(w http.ResponseWriter, r *http.Request) {
	uid := strings.TrimPrefix(r.URL.Path, "/order/")
	if uid == "" {
		http.Error(w, "order id required", http.StatusBadRequest)
		return
	}

	order, st, err := s.service.GetByUIDWithStats(r.Context(), uid)
	if err != nil {
		// Можно дополнительно различать 404 и 500 по типу ошибки, если хранилище отдаёт ErrNotFound.
		http.Error(w, "no order with this id", http.StatusNotFound)
		return
	}

	observability.AppendServerTiming(w, "cache", st.CacheMs, "")
	observability.AppendServerTiming(w, "db", st.DBMs, "")
	observability.AppendServerTiming(w, "source", 0, string(st.Source))
	w.Header().Set("X-Source", string(st.Source))
	observability.SetIfPos(w, "X-Cache-Time", st.CacheMs)
	observability.SetIfPos(w, "X-DB-Time", st.DBMs)

	writeJSON(w, order)
}

func (s *Server) upsertOrder(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var order domain.Order
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&order); err != nil {
		s.logger.Error(
			"Error while decoding JSON",
			zap.Error(err),
		)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	if err := validateOrder(order); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	st, err := s.service.UpsertWithStats(r.Context(), &order)
	if err != nil {
		http.Error(w, "Service error", http.StatusInternalServerError)
		return
	}

	observability.AppendServerTiming(w, "db_write", st.DBWriteMs, "")

	writeJSON(w, order)
}

func validateOrder(order domain.Order) error {
	if order.OrderUID == "" {
		return errors.New("order_uid is required")
	}
	// ... some new validate params
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	// Connect middleware
	handler := ServerTimingApp(s.metrics)(s.mux)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	return srv.ListenAndServe()
}

func (s *Server) Handler() http.Handler { return s.mux }
