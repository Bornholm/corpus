package authz

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/pkg/errors"
)

type AssertFunc func(ctx context.Context, user model.User) (bool, error)

func IsAuthenticated(ctx context.Context, user model.User) (bool, error) {
	return user != nil, nil
}

func Is(username, provider string) AssertFunc {
	return func(ctx context.Context, user model.User) (bool, error) {
		return user != nil && user.Provider() == provider && user.Username() == username, nil
	}
}

func Has(role string) AssertFunc {
	return func(ctx context.Context, user model.User) (bool, error) {
		return user != nil && slices.Contains(user.Roles(), role), nil
	}
}

func OneOf(funcs ...AssertFunc) AssertFunc {
	return func(ctx context.Context, user model.User) (bool, error) {
		for _, fn := range funcs {
			allowed, err := fn(ctx, user)
			if err != nil {
				return false, errors.WithStack(err)
			}

			if allowed {
				return true, nil
			}
		}

		return false, nil
	}
}

func Assert(ctx context.Context, user model.User, funcs ...AssertFunc) (bool, error) {
	for _, fn := range funcs {
		allowed, err := fn(ctx, user)
		if err != nil {
			return false, errors.WithStack(err)
		}

		if !allowed {
			return false, nil
		}
	}

	return true, nil
}

func Middleware(funcs ...AssertFunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			user := httpCtx.User(ctx)

			allowed, err := Assert(ctx, user, funcs...)
			if err != nil {
				slog.ErrorContext(ctx, "could not assert user authorizations", slog.Any("error", errors.WithStack(err)))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			h.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}
