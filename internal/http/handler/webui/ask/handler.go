package ask

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/genai/llm"
)

type Handler struct {
	mux             *http.ServeMux
	documentManager *service.DocumentManager
	llm             llm.Client
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, llm llm.Client) *Handler {
	h := &Handler{
		mux:             http.NewServeMux(),
		documentManager: documentManager,
		llm:             llm,
	}

	h.mux.HandleFunc("GET /", h.getAskPage)
	h.mux.HandleFunc("POST /", h.handleAsk)
	h.mux.HandleFunc("POST /index", h.handleIndex)
	h.mux.HandleFunc("GET /tasks/{taskID}", h.getTaskPage)

	return h
}

var _ http.Handler = &Handler{}
