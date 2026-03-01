package collection

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
)

type Handler struct {
	mux             *http.ServeMux
	documentManager *service.DocumentManager
	userStore       port.UserStore
	taskRunner      port.TaskRunner
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, userStore port.UserStore, taskRunner port.TaskRunner) *Handler {
	h := &Handler{
		mux:             http.NewServeMux(),
		documentManager: documentManager,
		userStore:       userStore,
		taskRunner:      taskRunner,
	}

	assertUser := authz.Middleware(http.HandlerFunc(h.getForbiddenPage), authz.OneOf(authz.Has(authz.RoleUser), authz.Has(authz.RoleAdmin)))

	h.mux.Handle("GET /", assertUser(http.HandlerFunc(h.getCollectionListPage)))
	h.mux.Handle("GET /new", assertUser(http.HandlerFunc(h.getCollectionCreatePage)))
	h.mux.Handle("POST /new", assertUser(http.HandlerFunc(h.handleCollectionCreate)))
	h.mux.Handle("GET /{collectionID}/edit", assertUser(http.HandlerFunc(h.getCollectionEditPage)))
	h.mux.Handle("POST /{collectionID}/edit", assertUser(http.HandlerFunc(h.handleCollectionUpdate)))
	h.mux.Handle("DELETE /{collectionID}", assertUser(http.HandlerFunc(h.handleCollectionDelete)))
	h.mux.Handle("POST /{collectionID}/shares", assertUser(http.HandlerFunc(h.handleCollectionShareCreate)))
	h.mux.Handle("DELETE /{collectionID}/shares/{shareID}", assertUser(http.HandlerFunc(h.handleCollectionShareDelete)))
	h.mux.Handle("DELETE /{collectionID}/documents/{docID}", assertUser(http.HandlerFunc(h.handleDocumentDelete)))

	h.mux.Handle("POST /{collectionID}/index", assertUser(http.HandlerFunc(h.handleIndex)))
	h.mux.Handle("GET /{collectionID}/tasks/{taskID}", assertUser(http.HandlerFunc(h.getTaskPage)))

	return h
}

var _ http.Handler = &Handler{}
