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
	h.mux.HandleFunc("GET /ask", h.handleAsk)
	h.mux.HandleFunc("POST /index", h.handleIndexDocument)
	h.mux.HandleFunc("GET /tasks", h.listTasks)
	h.mux.HandleFunc("GET /tasks/{taskID}", h.showTask)

	return h
}

var _ http.Handler = &Handler{}
