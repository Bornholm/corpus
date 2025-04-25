package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"text/template"

	"github.com/bornholm/corpus/internal/core/model"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/log"
	"github.com/pkg/errors"
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

			if strings.HasSuffix(r.URL.Path, "/logout") {
				next.ServeHTTP(w, r)
				return
			}
		}

		if s.opts.AllowAnonymous && !strings.HasSuffix(r.URL.Path, "/login") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.opts.BaseURL, http.StatusSeeOther)
}

var logoutPageTemplate = template.Must(template.New("").Parse(`
<html>
	<head>
		<meta http-equiv="refresh" content="0; url={{ .RedirectURL }}" />
	</head>
	<body>
		<a href="{{ .RedirectURL }}">Redirection en cours...</a>
	</body>
</html>
`))

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	redirectURL := s.opts.BaseURL
	if referer := r.Referer(); referer != "" {
		if refererURL, err := url.Parse(referer); err == nil {
			refererURL.Path = ""
			refererURL.RawQuery = ""
			refererURL.User = nil
			redirectURL = refererURL.String()
		}
	}

	user := httpCtx.User(r.Context())
	if user != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		w.WriteHeader(http.StatusUnauthorized)
	}

	err := logoutPageTemplate.Execute(w, struct {
		RedirectURL string
	}{RedirectURL: redirectURL})
	if err != nil {
		slog.ErrorContext(r.Context(), "could not execute template", slog.Any("error", errors.WithStack(err)))
	}
}
