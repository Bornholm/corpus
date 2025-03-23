package ask

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/bornholm/corpus/internal/core/service"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/pkg/errors"
)

const maxBodySize = 32<<20 + 512

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		slog.ErrorContext(ctx, "could not parse multipart form", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		slog.ErrorContext(ctx, "could not read form file", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	options := make([]service.DocumentManagerIndexFileOptionFunc, 0)

	rawSource := r.FormValue("source")
	if rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			slog.ErrorContext(ctx, "could not parse source url", slog.Any("error", errors.WithStack(err)))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileSource(source))
	}

	if collections, exists := r.Form["collection"]; exists {
		sanitizedCollections := sanitizeCollections(collections)
		if len(sanitizedCollections) != len(collections) {
			common.HandleError(w, r, common.NewError("invalid collections", "Invalid collections", http.StatusBadRequest))
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileCollections(sanitizedCollections...))
	}

	slog.DebugContext(ctx, "indexing uploaded document")

	_, err = h.documentManager.IndexFile(ctx, fileHeader.Filename, file, options...)
	if err != nil {
		slog.ErrorContext(ctx, "could not index uploaded file", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	baseURL := httpCtx.BaseURL(ctx)

	http.Redirect(w, r, baseURL.String(), http.StatusSeeOther)
}

func sanitizeCollections(collections []string) []string {
	sanitized := make([]string, 0)
	for _, c := range collections {
		s := strings.ToLower(strings.TrimSpace(c))
		if s == "" {
			continue
		}

		sanitized = append(sanitized, s)
	}
	return sanitized
}
