package pubshare

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

type contextKey string

const (
	pubShareContextKey contextKey = "pubShare"
)

func ctxPubShare(ctx context.Context) model.PersistedPublicShare {
	raw := ctx.Value(pubShareContextKey)
	if raw == nil {
		panic(errors.New("no public share in context"))
	}

	pubShare, ok := raw.(model.PersistedPublicShare)
	if !ok {
		panic(errors.Errorf("unexpected context value type '%T'", raw))
	}

	return pubShare
}

func (h *Handler) assertToken(next http.HandlerFunc) http.Handler {
	var fn http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("publicShareToken")

		ctx := r.Context()

		publicShare, err := h.pubShareStore.FindPublicShareByToken(ctx, token)
		if err != nil {
			if errors.Is(err, port.ErrNotFound) {
				common.HandleError(w, r, common.NewHTTPError(http.StatusNotFound))
				return
			}

			slog.ErrorContext(ctx, "could not find public share", slogx.Error(err))
			common.HandleError(w, r, err)
			return
		}

		ctx = context.WithValue(ctx, pubShareContextKey, publicShare)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}
	return fn
}
