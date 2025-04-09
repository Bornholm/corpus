package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/log"
)

func (s *Server) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))

			userIndex := slices.IndexFunc(s.opts.Auth.Users, func(u User) bool {
				return u.Username == username
			})

			if userIndex != -1 {
				user := s.opts.Auth.Users[userIndex]

				expectedUsername := sha256.Sum256([]byte(user.Username))
				expectedPassword := sha256.Sum256([]byte(user.Password))

				usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsername[:]) == 1)
				passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPassword[:]) == 1)

				if usernameMatch && passwordMatch {
					user := model.NewReadOnlyUser(username, "basic-auth", user.Roles...)

					ctx := httpCtx.SetUser(r.Context(), user)
					ctx = log.WithAttrs(ctx, slog.String("user", model.UserString(user)))

					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			if s.opts.AllowAnonymous && !strings.HasSuffix(r.URL.Path, "/login") {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.opts.BaseURL, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.opts.BaseURL, http.StatusSeeOther)
}
