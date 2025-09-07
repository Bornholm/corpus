package api

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/authz"
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

	assertAuthenticated := authz.Middleware(nil, authz.IsAuthenticated)
	assertWriter := authz.Middleware(nil, authz.Has(authz.RoleWriter))

	h.mux.Handle("GET /search", assertAuthenticated(http.HandlerFunc(h.handleSearch)))
	h.mux.Handle("GET /ask", assertAuthenticated(http.HandlerFunc(h.handleAsk)))
	h.mux.Handle("POST /index", assertWriter(http.HandlerFunc(h.handleIndexDocument)))
	h.mux.Handle("GET /tasks", assertAuthenticated(http.HandlerFunc(h.listTasks)))
	h.mux.Handle("GET /tasks/{taskID}", assertAuthenticated(http.HandlerFunc(h.showTask)))

	h.mux.Handle("GET /backup", assertAuthenticated(http.HandlerFunc(h.handleGenerateBackup)))
	h.mux.Handle("PUT /backup", assertWriter(http.HandlerFunc(h.handleRestoreBackup)))

	h.mux.Handle("GET /documents", assertAuthenticated(http.HandlerFunc(h.handleListDocuments)))
	h.mux.Handle("GET /documents/{documentID}", assertAuthenticated(http.HandlerFunc(h.handleGetDocument)))
	h.mux.Handle("DELETE /documents/{documentID}", assertWriter(http.HandlerFunc(h.handleDeleteDocument)))
	h.mux.Handle("GET /documents/{documentID}/content", assertAuthenticated(http.HandlerFunc(h.handleGetDocumentContent)))
	h.mux.Handle("POST /documents/{documentID}/reindex", assertWriter(http.HandlerFunc(h.handleReindexDocument)))
	h.mux.Handle("GET /documents/{documentID}/sections/{sectionID}", assertAuthenticated(http.HandlerFunc(h.handleGetDocumentSection)))
	h.mux.Handle("GET /documents/{documentID}/sections/{sectionID}/content", assertAuthenticated(http.HandlerFunc(h.handleGetSectionContent)))

	return h
}

var _ http.Handler = &Handler{}
