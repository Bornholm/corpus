package collection

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
)

type Handler struct {
	mux             *http.ServeMux
	documentManager *service.DocumentManager
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager) *Handler {
	h := &Handler{
		mux:             http.NewServeMux(),
		documentManager: documentManager,
	}

	assertUser := authz.Middleware(http.HandlerFunc(h.getForbiddenPage), authz.OneOf(authz.Has(authz.RoleUser), authz.Has(authz.RoleAdmin)))

	h.mux.Handle("GET /", assertUser(http.HandlerFunc(h.getCollectionListPage)))
	h.mux.Handle("GET /new", assertUser(http.HandlerFunc(h.getCollectionCreatePage)))
	h.mux.Handle("POST /new", assertUser(http.HandlerFunc(h.handleCollectionCreate)))
	h.mux.Handle("GET /{id}/edit", assertUser(http.HandlerFunc(h.getCollectionEditPage)))
	h.mux.Handle("POST /{id}/edit", assertUser(http.HandlerFunc(h.handleCollectionUpdate)))
	h.mux.Handle("DELETE /{id}", assertUser(http.HandlerFunc(h.handleCollectionDelete)))

	return h
}

var _ http.Handler = &Handler{}
