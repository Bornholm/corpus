package component

import (
	"context"

	"github.com/a-h/templ"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/url"
)

var (
	WithPath        = url.WithPath
	WithoutValues   = url.WithoutValues
	WithValuesReset = url.WithValuesReset
	WithValues      = url.WithValues
)

func BaseURL(ctx context.Context, funcs ...url.MutationFunc) templ.SafeURL {
	baseURL := httpCtx.BaseURL(ctx)
	mutated := url.Mutate(baseURL, funcs...)
	return templ.SafeURL(mutated.String())
}

func CurrentURL(ctx context.Context, funcs ...url.MutationFunc) templ.SafeURL {
	currentURL := clone(httpCtx.CurrentURL(ctx))
	mutated := url.Mutate(currentURL, funcs...)
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
