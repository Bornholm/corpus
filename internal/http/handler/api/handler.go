package api

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/service"
)

type Handler struct {
	documentManager *service.DocumentManager
	mux             *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager) *Handler {
	h := &Handler{
		documentManager: documentManager,
		mux:             &http.ServeMux{},
	}

	h.mux.HandleFunc("GET /search", h.handleSearch)
	h.mux.HandleFunc("POST /index", h.handleIndexDocument)

	return h
}

var _ http.Handler = &Handler{}
