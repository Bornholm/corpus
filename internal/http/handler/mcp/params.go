package mcp

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

func (h *Handler) withParams(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/mcp/sse") {
			next.ServeHTTP(w, r)
			return
		}

		shouldSave := false

		sessionData := h.getSession(r)

		query := r.URL.Query()

		if collections := query["collection"]; len(collections) > 0 {
			sessionData.Collections = collections
			shouldSave = true
		}

		if shouldSave {
			if err := h.saveSession(w, r, sessionData); err != nil {
				slog.ErrorContext(r.Context(), "could not save session", slog.Any("error", errors.WithStack(err)))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
