package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/TemirB/WB-TECH-L0/internal/application/query"
	"github.com/TemirB/WB-TECH-L0/internal/domain"
	"github.com/TemirB/WB-TECH-L0/internal/shared"
)

type Handler struct {
	getOrder *query.GetOrderHandler
}

func NewHandler(get *query.GetOrderHandler) *Handler {
	return &Handler{
		getOrder: get,
	}
}

func (h *Handler) GetOrderByID(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "order_uid")
	if uid == "" {
		http.Error(w, "missing order_uid", http.StatusBadRequest)
		return
	}
	order, err := h.getOrder.Handle(r.Context(), uid)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	shared.WriteJSON(w, order)
}
