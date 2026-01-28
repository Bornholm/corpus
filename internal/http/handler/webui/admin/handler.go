package admin

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
)

type Handler struct {
	mux              *http.ServeMux
	userStore        port.UserStore
	documentStore    port.DocumentStore
	publicShareStore port.PublicShareStore
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(userStore port.UserStore, documentStore port.DocumentStore, publicShareStore port.PublicShareStore) *Handler {
	h := &Handler{
		mux:              http.NewServeMux(),
		userStore:        userStore,
		documentStore:    documentStore,
		publicShareStore: publicShareStore,
	}

	// Admin middleware - only allow admin users
	assertAdmin := authz.Middleware(http.HandlerFunc(h.getForbiddenPage), authz.Has(authz.RoleAdmin))

	h.mux.Handle("GET /", assertAdmin(http.HandlerFunc(h.getIndexPage)))
	h.mux.Handle("GET /public-shares", assertAdmin(http.HandlerFunc(h.getPublicSharesPage)))
	h.mux.Handle("GET /public-shares/new", assertAdmin(http.HandlerFunc(h.getNewPublicSharePage)))
	h.mux.Handle("POST /public-shares", assertAdmin(http.HandlerFunc(h.postPublicShare)))
	h.mux.Handle("GET /public-shares/{id}/edit", assertAdmin(http.HandlerFunc(h.getEditPublicSharePage)))
	h.mux.Handle("POST /public-shares/{id}/edit", assertAdmin(http.HandlerFunc(h.postEditPublicShare)))
	h.mux.Handle("DELETE /public-shares/{id}", assertAdmin(http.HandlerFunc(h.handlePublicShareDelete)))

	return h
}

func (h *Handler) getForbiddenPage(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}

var _ http.Handler = &Handler{}
