package api

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/authz"
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

	assertAuthenticated := authz.Middleware(authz.IsAuthenticated)
	assertWriter := authz.Middleware(authz.Has(authz.RoleWriter))

	h.mux.Handle("GET /search", assertAuthenticated(http.HandlerFunc(h.handleSearch)))
	h.mux.Handle("GET /ask", assertAuthenticated(http.HandlerFunc(h.handleAsk)))
	h.mux.Handle("POST /index", assertWriter(http.HandlerFunc(h.handleIndexDocument)))
	h.mux.Handle("GET /tasks", assertAuthenticated(http.HandlerFunc(h.listTasks)))
	h.mux.Handle("GET /tasks/{taskID}", assertAuthenticated(http.HandlerFunc(h.showTask)))

	return h
}

var _ http.Handler = &Handler{}
