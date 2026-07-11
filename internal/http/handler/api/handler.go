package api

import (
	"net/http"

	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/core/service/backup"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
)

type Handler struct {
	documentManager       *service.DocumentManager
	backupManager         *backup.Manager
	taskRunner            port.TaskRunner
	filesystemSourceStore port.FilesystemSourceStore
	mux                   *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, backupManager *backup.Manager, taskRunner port.TaskRunner, filesystemSourceStore port.FilesystemSourceStore) *Handler {
	h := &Handler{
		documentManager:       documentManager,
		backupManager:         backupManager,
		taskRunner:            taskRunner,
		filesystemSourceStore: filesystemSourceStore,
		mux:                   &http.ServeMux{},
	}

	assertUser := authz.Middleware(nil, authz.OneOf(authz.Has(authz.RoleUser), authz.Has(authz.RoleAdmin)))
	assertAdmin := authz.Middleware(nil, authz.Has(authz.RoleAdmin))

	h.mux.Handle("GET /search", assertUser(http.HandlerFunc(h.handleSearch)))
	h.mux.Handle("GET /ask", assertUser(http.HandlerFunc(h.handleAsk)))
	h.mux.Handle("POST /index", assertUser(http.HandlerFunc(h.handleIndexDocument)))
	h.mux.Handle("GET /tasks", assertUser(http.HandlerFunc(h.listTasks)))
	h.mux.Handle("GET /tasks/{taskID}", assertUser(http.HandlerFunc(h.showTask)))

	h.mux.Handle("GET /backup", assertAdmin(http.HandlerFunc(h.handleGenerateBackup)))
	h.mux.Handle("PUT /backup", assertAdmin(http.HandlerFunc(h.handleRestoreBackup)))

	h.mux.Handle("GET /documents/digests", assertUser(http.HandlerFunc(h.handleListDocumentDigests)))
	h.mux.Handle("GET /documents", assertUser(http.HandlerFunc(h.handleListDocuments)))
	h.mux.Handle("GET /documents/{documentID}", assertUser(h.assertDocumentReadable(http.HandlerFunc(h.handleGetDocument))))
	h.mux.Handle("DELETE /documents/{documentID}", assertUser(h.assertDocumentWritable(http.HandlerFunc(h.handleDeleteDocument))))
	h.mux.Handle("GET /documents/{documentID}/content", assertUser(h.assertDocumentReadable(http.HandlerFunc(h.handleGetDocumentContent))))
	h.mux.Handle("POST /documents/{documentID}/reindex", assertUser(h.assertDocumentWritable(http.HandlerFunc(h.handleReindexDocument))))
	h.mux.Handle("GET /documents/{documentID}/sections/{sectionID}", assertUser(h.assertDocumentReadable(http.HandlerFunc(h.handleGetDocumentSection))))
	h.mux.Handle("GET /documents/{documentID}/sections/{sectionID}/content", assertUser(h.assertDocumentReadable(http.HandlerFunc(h.handleGetSectionContent))))

	h.mux.Handle("GET /collections", assertUser(http.HandlerFunc(h.handleListCollections)))
	h.mux.Handle("GET /collections/{collectionID}", assertUser(h.assertCollectionReadable(http.HandlerFunc(h.handleGetCollection))))
	h.mux.Handle("PUT /collections/{collectionID}", assertUser(h.assertCollectionWritable(http.HandlerFunc(h.handleUpdateCollection))))
	h.mux.Handle("DELETE /collections/{collectionID}", assertUser(h.assertCollectionWritable(http.HandlerFunc(h.handleDeleteCollection))))
	h.mux.Handle("GET /collections/{collectionID}/shares", assertUser(http.HandlerFunc(h.handleListCollectionShares)))
	h.mux.Handle("POST /collections/{collectionID}/shares", assertUser(http.HandlerFunc(h.handleCreateCollectionShare)))
	h.mux.Handle("DELETE /collections/{collectionID}/shares/{shareID}", assertUser(http.HandlerFunc(h.handleDeleteCollectionShare)))

	h.mux.Handle("GET /filesystem-sources/backend-schemas", assertAdmin(http.HandlerFunc(h.handleGetFilesystemBackendSchemas)))
	h.mux.Handle("GET /filesystem-sources", assertAdmin(http.HandlerFunc(h.handleListFilesystemSources)))
	h.mux.Handle("POST /filesystem-sources", assertAdmin(http.HandlerFunc(h.handleCreateFilesystemSource)))
	h.mux.Handle("GET /filesystem-sources/{sourceID}", assertAdmin(http.HandlerFunc(h.handleGetFilesystemSource)))
	h.mux.Handle("PUT /filesystem-sources/{sourceID}", assertAdmin(http.HandlerFunc(h.handleUpdateFilesystemSource)))
	h.mux.Handle("DELETE /filesystem-sources/{sourceID}", assertAdmin(http.HandlerFunc(h.handleDeleteFilesystemSource)))
	h.mux.Handle("POST /filesystem-sources/{sourceID}/sync", assertAdmin(http.HandlerFunc(h.handleSyncFilesystemSource)))

	return h
}

var _ http.Handler = &Handler{}
