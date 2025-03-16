package ask

import (
	"net/http"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/genai/llm"
)

type Handler struct {
	mux   *http.ServeMux
	index port.Index
	store port.Store
	llm   llm.Client
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(index port.Index, store port.Store, llm llm.Client) *Handler {
	h := &Handler{
		mux:   http.NewServeMux(),
		index: index,
		store: store,
		llm:   llm,
	}

	h.mux.HandleFunc("GET /", h.getAskPage)
	h.mux.HandleFunc("POST /", h.handleAsk)

	return h
}

var _ http.Handler = &Handler{}
