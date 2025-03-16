package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/bornholm/corpus/internal/markdown"
	"github.com/bornholm/corpus/internal/workflow"
	"github.com/pkg/errors"
)

const maxBodySize = 32<<20 + 512

func (h *Handler) handleIndexDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		slog.ErrorContext(ctx, "could not parse multipart form", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		slog.ErrorContext(ctx, "could not read form file", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		slog.ErrorContext(ctx, "could not read file", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var source *url.URL

	rawSource := r.FormValue("source")
	if rawSource != "" {
		source, err = url.Parse(rawSource)
		if err != nil {
			slog.ErrorContext(ctx, "could not parse source url", slog.Any("error", errors.WithStack(err)))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}

	collection := r.FormValue("collection")

	slog.DebugContext(ctx, "indexing uploaded document")

	if err := h.indexUploadedDocument(ctx, collection, source, data); err != nil {
		slog.ErrorContext(ctx, "could not execute index workflow", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	slog.DebugContext(ctx, "uploaded document indexed")

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) indexUploadedDocument(ctx context.Context, collection string, source *url.URL, data []byte) error {
	var document *markdown.Document

	wf := workflow.New(
		workflow.StepFunc(
			func(ctx context.Context) error {
				doc, err := markdown.Parse(data)
				if err != nil {
					return errors.Wrap(err, "could not build document")
				}

				if source == nil {
					source = doc.Source()
				} else {
					doc.SetSource(source)
				}

				if source == nil {
					return errors.New("document source missing")
				}

				doc.SetCollection(collection)

				document = doc

				return nil
			},
			nil,
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				if err := h.store.SaveDocument(ctx, document); err != nil {
					return errors.WithStack(err)
				}

				return nil
			},
			func(ctx context.Context) error {
				if err := h.store.DeleteDocumentBySource(ctx, document.Source()); err != nil {
					return errors.WithStack(err)
				}

				return nil
			},
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				if err := h.index.Index(ctx, document); err != nil {
					return errors.WithStack(err)
				}

				return nil
			},
			func(ctx context.Context) error {
				if err := h.index.DeleteBySource(ctx, document.Source()); err != nil {
					return errors.WithStack(err)
				}

				return nil
			},
		),
	)
	if err := wf.Execute(ctx); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
