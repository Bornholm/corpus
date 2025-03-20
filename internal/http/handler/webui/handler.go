package webui

import (
	"net/http"
	"strings"

	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/handler/webui/ask"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/corpus/internal/http/handler/webui/swagger"
	"github.com/bornholm/genai/llm"
)

type Handler struct {
	mux *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, llm llm.Client) *Handler {
	mux := http.NewServeMux()

	mount(mux, "/", ask.NewHandler(documentManager, llm))
	mount(mux, "/docs/", swagger.NewHandler())
	mount(mux, "/assets/", common.NewHandler())

	h := &Handler{
		mux: mux,
	}

	return h
}

func mount(mux *http.ServeMux, prefix string, handler http.Handler) {
	trimmed := strings.TrimSuffix(prefix, "/")

	if len(trimmed) > 0 {
		mux.Handle(prefix, http.StripPrefix(trimmed, handler))
	} else {
		mux.Handle(prefix, handler)
	}
}

var _ http.Handler = &Handler{}
