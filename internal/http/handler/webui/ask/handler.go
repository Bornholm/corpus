package ask

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
	"github.com/bornholm/genai/llm"
)

type Handler struct {
	mux             *http.ServeMux
	documentManager *service.DocumentManager
	taskRunner      port.TaskRunner
	llm             llm.Client
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, llm llm.Client, taskRunner port.TaskRunner) *Handler {
	h := &Handler{
		mux:             http.NewServeMux(),
		documentManager: documentManager,
		taskRunner:      taskRunner,
		llm:             llm,
	}

	assertUser := authz.Middleware(http.HandlerFunc(h.getForbiddenPage), authz.OneOf(authz.Has(authz.RoleUser), authz.Has(authz.RoleAdmin)))

	h.mux.Handle("GET /", assertUser(http.HandlerFunc(h.getAskPage)))
	h.mux.Handle("POST /", assertUser(http.HandlerFunc(h.handleAsk)))
	h.mux.Handle("POST /index", assertUser(http.HandlerFunc(h.handleIndex)))
	h.mux.Handle("GET /tasks/{taskID}", assertUser(http.HandlerFunc(h.getTaskPage)))

	return h
}

var _ http.Handler = &Handler{}
