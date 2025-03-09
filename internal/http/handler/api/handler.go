package api

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
)

type Handler struct {
	store port.Store
	index port.Index
	mux   *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(store port.Store, index port.Index) *Handler {
	h := &Handler{
		index: index,
		store: store,
		mux:   &http.ServeMux{},
	}

	h.mux.HandleFunc("GET /search", h.handleSearch)
	h.mux.HandleFunc("POST /index", h.handleIndexDocument)

	return h
}

var _ http.Handler = &Handler{}
