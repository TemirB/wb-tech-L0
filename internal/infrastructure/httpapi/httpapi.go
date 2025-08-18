package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/TemirB/wb-tech-L0/internal/application"
	"github.com/TemirB/wb-tech-L0/internal/domain"
	"github.com/TemirB/wb-tech-L0/internal/shared"

	"github.com/go-chi/chi/v5"
)

type Service interface {
	GetByUID(ctx context.Context, uid string) (*domain.Order, error)
}

type Handler struct {
	service Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "order_uid")
	if uid == "" {
		http.Error(w, "missing order_uid", http.StatusBadRequest)
		return
	}
	o, err := h.service.GetByUID(r.Context(), uid)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	shared.WriteJSON(w, o)
}
