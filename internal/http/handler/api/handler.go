package api

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
)

type Handler struct {
	documentManager *service.DocumentManager
	backupManager   *service.BackupManager
	taskRunner      port.TaskRunner
	mux             *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, backupManager *service.BackupManager, taskRunner port.TaskRunner) *Handler {
	h := &Handler{
		documentManager: documentManager,
		backupManager:   backupManager,
		taskRunner:      taskRunner,
		mux:             &http.ServeMux{},
	}

	assertUser := authz.Middleware(nil, authz.Has(authz.RoleUser))
	assertAdmin := authz.Middleware(nil, authz.Has(authz.RoleAdmin))

	h.mux.Handle("GET /search", assertUser(http.HandlerFunc(h.handleSearch)))
	h.mux.Handle("GET /ask", assertUser(http.HandlerFunc(h.handleAsk)))
	h.mux.Handle("POST /index", assertUser(http.HandlerFunc(h.handleIndexDocument)))
	h.mux.Handle("GET /tasks", assertUser(http.HandlerFunc(h.listTasks)))
	h.mux.Handle("GET /tasks/{taskID}", assertUser(http.HandlerFunc(h.showTask)))

	h.mux.Handle("GET /backup", assertAdmin(http.HandlerFunc(h.handleGenerateBackup)))
	h.mux.Handle("PUT /backup", assertAdmin(http.HandlerFunc(h.handleRestoreBackup)))

	h.mux.Handle("GET /documents", assertUser(http.HandlerFunc(h.handleListDocuments)))
	h.mux.Handle("GET /documents/{documentID}", assertUser(http.HandlerFunc(h.handleGetDocument)))
	h.mux.Handle("DELETE /documents/{documentID}", assertUser(http.HandlerFunc(h.handleDeleteDocument)))
	h.mux.Handle("GET /documents/{documentID}/content", assertUser(http.HandlerFunc(h.handleGetDocumentContent)))
	h.mux.Handle("POST /documents/{documentID}/reindex", assertUser(http.HandlerFunc(h.handleReindexDocument)))
	h.mux.Handle("GET /documents/{documentID}/sections/{sectionID}", assertUser(http.HandlerFunc(h.handleGetDocumentSection)))
	h.mux.Handle("GET /documents/{documentID}/sections/{sectionID}/content", assertUser(http.HandlerFunc(h.handleGetSectionContent)))

	return h
}

var _ http.Handler = &Handler{}
