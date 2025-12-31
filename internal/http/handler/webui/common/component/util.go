package component

import (
	"context"
	"net/url"

	"github.com/a-h/templ"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
	httpURL "github.com/bornholm/corpus/internal/http/url"
	"github.com/pkg/errors"
)

var (
	WithPath        = httpURL.WithPath
	WithoutValues   = httpURL.WithoutValues
	WithValuesReset = httpURL.WithValuesReset
	WithValues      = httpURL.WithValues
)

func IsDesktopApp(ctx context.Context) bool {
	return httpCtx.IsDesktopApp(ctx)
}

func WithUser(username string, password string) httpURL.MutationFunc {
	return func(u *url.URL) {
		u.User = url.UserPassword(username, password)
	}
}

func BaseURL(ctx context.Context, funcs ...httpURL.MutationFunc) templ.SafeURL {
	baseURL := httpCtx.BaseURL(ctx)
	mutated := httpURL.Mutate(baseURL, funcs...)
	return templ.SafeURL(mutated.String())
}

func CurrentURL(ctx context.Context, funcs ...httpURL.MutationFunc) templ.SafeURL {
	currentURL := clone(httpCtx.CurrentURL(ctx))
	mutated := httpURL.Mutate(currentURL, funcs...)
	return templ.SafeURL(mutated.String())
}

func MatchPath(ctx context.Context, path string) bool {
	currentURL := httpCtx.CurrentURL(ctx)
	return currentURL.Path == path
}

func clone[T any](v *T) *T {
	copy := *v
	return &copy
}

func AssertUser(ctx context.Context, funcs ...authz.AssertFunc) bool {
	user := httpCtx.User(ctx)
	if user == nil {
		return false
	}

	allowed, err := authz.Assert(ctx, user, funcs...)
	if err != nil {
		panic(errors.WithStack(err))
	}

	return allowed
}

var User = httpCtx.User
