package setup

import (
	"context"
	"net/http"

	"github.com/bornholm/corpus/internal/desktop"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
)

func NewDesktopServer(ctx context.Context) (*http.Server, error) {
	var handler http.Handler = desktop.NewHandler()

	handler = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ctx = httpCtx.SetBaseURL(ctx, "/")
			ctx = httpCtx.SetCurrentURL(ctx, r.URL)

			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}(handler)

	server := &http.Server{
		Handler: handler,
	}

	return server, nil
}
