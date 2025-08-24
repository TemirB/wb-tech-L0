package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/TemirB/wb-tech-L0/internal/domain"
	"go.uber.org/zap"
)

//go:generate mockgen -destination=../mocks/mock_service.go -package=mocks github.com/TemirB/wb-tech-L0/internal/httpapi Service

type Service interface {
	GetByUID(ctx context.Context, uid string) (*domain.Order, error)
	Upsert(ctx context.Context, order *domain.Order) error
}

type Server struct {
	service Service
	mux     *http.ServeMux
	logger  *zap.Logger
}

func New(service Service, logger *zap.Logger) *Server {
	s := &Server{
		service: service,
		logger:  logger,
		mux:     http.NewServeMux(),
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

	if order, err := s.service.GetByUID(r.Context(), uid); err == nil {
		writeJSON(w, order)
		return
	}

	http.Error(w, "no order with this id", http.StatusNotFound)
	writeJSON(w, nil)
}

func (s *Server) upsertOrder(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
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
	}

	if err := validateOrder(order); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.service.Upsert(r.Context(), &order); err != nil {
		http.Error(w, "Service error", http.StatusInternalServerError)
		writeJSON(w, nil)
	}

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
	srv := &http.Server{Addr: addr, Handler: s.mux}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	return srv.ListenAndServe()
}

func (s *Server) Handler() http.Handler { return s.mux }
