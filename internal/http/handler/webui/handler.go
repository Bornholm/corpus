package webui

import (
	"net/http"
	"strings"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/handler/webui/admin"
	"github.com/bornholm/corpus/internal/http/handler/webui/ask"
	"github.com/bornholm/corpus/internal/http/handler/webui/collection"
	"github.com/bornholm/corpus/internal/http/handler/webui/profile"
	"github.com/bornholm/corpus/internal/http/handler/webui/swagger"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
	"github.com/bornholm/genai/llm"
)

type Handler struct {
	mux *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(documentManager *service.DocumentManager, llm llm.Client, taskRunner port.TaskRunner, userStore port.UserStore, documentStore port.DocumentStore, publicShareStore port.PublicShareStore) *Handler {

	h := &Handler{
		mux: http.NewServeMux(),
	}

	isActive := authz.Middleware(http.HandlerFunc(h.getInactiveUserPage), authz.Active())

	mount(h.mux, "/", isActive(ask.NewHandler(documentManager, llm, taskRunner)))
	mount(h.mux, "/collections/", isActive(collection.NewHandler(documentManager, userStore)))
	mount(h.mux, "/profile/", isActive((profile.NewHandler(userStore))))
	mount(h.mux, "/admin/", isActive(admin.NewHandler(userStore, documentStore, publicShareStore, taskRunner, documentManager)))
	mount(h.mux, "/docs/", swagger.NewHandler())

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
