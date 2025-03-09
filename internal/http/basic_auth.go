package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

func (s *Server) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))

			expectedUsername := sha256.Sum256([]byte(s.opts.BasicAuth.Username))
			expectedPassword := sha256.Sum256([]byte(s.opts.BasicAuth.Password))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsername[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPassword[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
