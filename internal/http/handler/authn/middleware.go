package authn

import (
	"log/slog"
	"net/http"

	"github.com/pkg/errors"
)

var ErrUserNotFound = errors.New("user not found")

func (h *Handler) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			user, err := h.retrieveSessionUser(r)
			if err != nil {
				slog.ErrorContext(r.Context(), "could not retrieve user from session", slog.Any("error", errors.WithStack(err)))
				http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
				return
			}

			ctx := r.Context()
			ctx = setContextUser(ctx, user)

			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}
