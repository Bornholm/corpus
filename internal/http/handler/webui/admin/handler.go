package admin

import (
	"context"
	"net/http"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
)

type DocumentManagerInterface interface {
	ReindexCollection(ctx context.Context, owner model.User, collectionID model.CollectionID) (model.TaskID, error)
	QueryUserWritableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error)
}

type Handler struct {
	mux              *http.ServeMux
	userStore        port.UserStore
	documentStore    port.DocumentStore
	publicShareStore port.PublicShareStore
	taskRunner       port.TaskRunner
	documentManager  DocumentManagerInterface
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(userStore port.UserStore, documentStore port.DocumentStore, publicShareStore port.PublicShareStore, taskRunner port.TaskRunner, documentManager DocumentManagerInterface) *Handler {
	h := &Handler{
		mux:              http.NewServeMux(),
		userStore:        userStore,
		documentStore:    documentStore,
		publicShareStore: publicShareStore,
		taskRunner:       taskRunner,
		documentManager:  documentManager,
	}

	// Admin middleware - only allow admin users
	assertAdmin := authz.Middleware(http.HandlerFunc(h.getForbiddenPage), authz.Has(authz.RoleAdmin))

	h.mux.Handle("GET /", assertAdmin(http.HandlerFunc(h.getIndexPage)))

	// User routes
	h.mux.Handle("GET /users", assertAdmin(http.HandlerFunc(h.getUsersPage)))
	h.mux.Handle("GET /users/{id}/edit", assertAdmin(http.HandlerFunc(h.getEditUserPage)))
	h.mux.Handle("POST /users/{id}/edit", assertAdmin(http.HandlerFunc(h.postEditUser)))

	h.mux.Handle("GET /public-shares", assertAdmin(http.HandlerFunc(h.getPublicSharesPage)))
	h.mux.Handle("GET /public-shares/new", assertAdmin(http.HandlerFunc(h.getNewPublicSharePage)))
	h.mux.Handle("POST /public-shares", assertAdmin(http.HandlerFunc(h.postPublicShare)))
	h.mux.Handle("GET /public-shares/{id}/edit", assertAdmin(http.HandlerFunc(h.getEditPublicSharePage)))
	h.mux.Handle("POST /public-shares/{id}/edit", assertAdmin(http.HandlerFunc(h.postEditPublicShare)))
	h.mux.Handle("DELETE /public-shares/{id}", assertAdmin(http.HandlerFunc(h.handlePublicShareDelete)))

	// Collection share routes
	h.mux.Handle("GET /collection-shares", assertAdmin(http.HandlerFunc(h.getCollectionSharesPage)))
	h.mux.Handle("POST /collection-shares", assertAdmin(http.HandlerFunc(h.postCollectionShare)))
	h.mux.Handle("DELETE /collection-shares/{id}", assertAdmin(http.HandlerFunc(h.handleCollectionShareDelete)))

	// Task routes
	h.mux.Handle("GET /tasks", assertAdmin(http.HandlerFunc(h.getTasksPage)))
	h.mux.Handle("GET /tasks/{id}", assertAdmin(http.HandlerFunc(h.getTaskPage)))
	h.mux.Handle("POST /tasks/{id}/cancel", assertAdmin(http.HandlerFunc(h.postCancelTask)))

	// Actions
	h.mux.Handle("POST /tasks/reindex", assertAdmin(http.HandlerFunc(h.postReindexCollection)))

	return h
}

func (h *Handler) getForbiddenPage(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}

var _ http.Handler = &Handler{}
