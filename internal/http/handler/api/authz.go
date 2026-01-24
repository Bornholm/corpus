package api

import (
	"log/slog"
	"net/http"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

func (h *Handler) assertDocumentReadable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := httpCtx.User(ctx)

		documentID := model.DocumentID(r.PathValue("documentID"))

		canRead, err := h.documentManager.CanReadDocument(ctx, user.ID(), documentID)
		if err != nil {
			if errors.Is(err, port.ErrNotFound) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}

			slog.ErrorContext(ctx, "could not check if document is readable", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !canRead {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) assertCollectionReadable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := httpCtx.User(ctx)

		collectionID := model.CollectionID(r.PathValue("collectionID"))

		canRead, err := h.documentManager.DocumentStore.CanReadCollection(ctx, user.ID(), collectionID)
		if err != nil {
			if errors.Is(err, port.ErrNotFound) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}

			slog.ErrorContext(ctx, "could not check if collection is readable", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !canRead {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) assertCollectionWritable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := httpCtx.User(ctx)

		collectionID := model.CollectionID(r.PathValue("collectionID"))

		canWrite, err := h.documentManager.DocumentStore.CanWriteCollection(ctx, user.ID(), collectionID)
		if err != nil {
			if errors.Is(err, port.ErrNotFound) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}

			slog.ErrorContext(ctx, "could not check if collection is writable", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !canWrite {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) assertDocumentWritable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := httpCtx.User(ctx)

		documentID := model.DocumentID(r.PathValue("documentID"))

		canWrite, err := h.documentManager.CanWriteDocument(ctx, user.ID(), documentID)
		if err != nil {
			if errors.Is(err, port.ErrNotFound) {
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}

			slog.ErrorContext(ctx, "could not check if document is writable", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !canWrite {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
