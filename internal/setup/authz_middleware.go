package setup

import (
	"context"
	"net/http"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/model"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/authn"
)

const (
	anonymousUser = "anonymous"
	wildcard      = "*"
)

func getAuthzMiddlewareFromConfig(ctx context.Context, conf *config.Config) (func(http.Handler) http.Handler, error) {
	whitelist := conf.HTTP.Authn.Whitelist
	defaultRole := conf.HTTP.Authn.DefaultRole
	roleMappings := conf.HTTP.Authn.RoleMappings

	hasRoleMappings := len(roleMappings) > 0
	findUserRole := func(email string) string {
		if !hasRoleMappings {
			return defaultRole
		}

		role, exists := roleMappings[email]
		if exists {
			return role
		}

		return defaultRole
	}

	indexedWhitelist := make(map[string]struct{}, len(whitelist))
	for _, e := range whitelist {
		indexedWhitelist[e] = struct{}{}
	}

	hasWhitelist := len(whitelist) > 0

	inWhitelist := func(email string) bool {
		if !hasWhitelist {
			return true
		}

		_, exists := indexedWhitelist[email]
		if exists {
			return true
		}

		return false
	}

	return func(next http.Handler) http.Handler {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			authnUser := authn.ContextUser(ctx)

			if authnUser == nil || !inWhitelist(authnUser.Email) {
				user := model.NewUser(authnUser.Provider, authnUser.Subject, authnUser.DisplayName, anonymousUser)
				ctx = httpCtx.SetUser(ctx, user)
				r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
				return
			}

			role := findUserRole(authnUser.Email)
			user := model.NewUser(authnUser.Provider, authnUser.Subject, authnUser.DisplayName, role)
			ctx = httpCtx.SetUser(ctx, user)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})

		return handler
	}, nil
}
