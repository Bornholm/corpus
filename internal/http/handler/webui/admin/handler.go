package admin

import (
	"context"
	"net/http"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
)

type DocumentManagerInterface interface {
	ReindexCollection(ctx context.Context, owner model.User, collectionID model.CollectionID) (model.TaskID, error)
	QueryUserWritableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error)
}

type Handler struct {
	mux                   *http.ServeMux
	userStore             port.UserStore
	documentStore         port.DocumentStore
	publicShareStore      port.PublicShareStore
	taskRunner            port.TaskRunner
	documentManager       DocumentManagerInterface
	filesystemSourceStore port.FilesystemSourceStore
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(userStore port.UserStore, documentStore port.DocumentStore, publicShareStore port.PublicShareStore, taskRunner port.TaskRunner, documentManager DocumentManagerInterface, filesystemSourceStore port.FilesystemSourceStore) *Handler {
	h := &Handler{
		mux:                   http.NewServeMux(),
		userStore:             userStore,
		documentStore:         documentStore,
		publicShareStore:      publicShareStore,
		taskRunner:            taskRunner,
		documentManager:       documentManager,
		filesystemSourceStore: filesystemSourceStore,
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

	// Collection share routes (for delete operations)
	h.mux.Handle("POST /collection-shares", assertAdmin(http.HandlerFunc(h.postCollectionShare)))
	h.mux.Handle("DELETE /collection-shares/{id}", assertAdmin(http.HandlerFunc(h.handleCollectionShareDelete)))

	// Collection routes
	h.mux.Handle("GET /collections", assertAdmin(http.HandlerFunc(h.getCollectionsPage)))
	h.mux.Handle("GET /collections/{id}", assertAdmin(http.HandlerFunc(h.getCollectionPage)))
	h.mux.Handle("POST /collections/{id}/reindex", assertAdmin(http.HandlerFunc(h.postReindexCollectionFromCollectionPage)))
	h.mux.Handle("DELETE /collections/{id}", assertAdmin(http.HandlerFunc(h.handleDeleteCollection)))

	// Task routes
	h.mux.Handle("GET /tasks", assertAdmin(http.HandlerFunc(h.getTasksPage)))
	h.mux.Handle("GET /tasks/{id}", assertAdmin(http.HandlerFunc(h.getTaskPage)))
	h.mux.Handle("POST /tasks/{id}/cancel", assertAdmin(http.HandlerFunc(h.postCancelTask)))

	// Actions
	h.mux.Handle("POST /tasks/reindex", assertAdmin(http.HandlerFunc(h.postReindexCollection)))

	// Filesystem source routes
	// NOTE: literal paths must come before {id} wildcard routes
	h.mux.Handle("GET /filesystem-sources/backend-form", assertAdmin(http.HandlerFunc(h.getBackendFormPartial)))
	h.mux.Handle("GET /filesystem-sources", assertAdmin(http.HandlerFunc(h.getFilesystemSourcesPage)))
	h.mux.Handle("GET /filesystem-sources/new", assertAdmin(http.HandlerFunc(h.getNewFilesystemSourcePage)))
	h.mux.Handle("POST /filesystem-sources", assertAdmin(http.HandlerFunc(h.postFilesystemSource)))
	h.mux.Handle("GET /filesystem-sources/{id}", assertAdmin(http.HandlerFunc(h.getFilesystemSourcePage)))
	h.mux.Handle("GET /filesystem-sources/{id}/edit", assertAdmin(http.HandlerFunc(h.getEditFilesystemSourcePage)))
	h.mux.Handle("POST /filesystem-sources/{id}", assertAdmin(http.HandlerFunc(h.postEditFilesystemSource)))
	h.mux.Handle("POST /filesystem-sources/{id}/delete", assertAdmin(http.HandlerFunc(h.postDeleteFilesystemSource)))
	h.mux.Handle("POST /filesystem-sources/{id}/sync", assertAdmin(http.HandlerFunc(h.postSyncFilesystemSource)))

	return h
}

func (h *Handler) getForbiddenPage(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Forbidden", http.StatusForbidden)
}

var _ http.Handler = &Handler{}
