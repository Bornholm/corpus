package api

import (
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/pkg/errors"
)

func (h *Handler) handleGenerateBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	slog.DebugContext(ctx, "generating backup")

	reader, err := h.documentManager.Backup(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not restore backup", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("corpus_backup_%d.bin.gz", time.Now().Unix())

	baseURL := httpCtx.BaseURL(ctx)

	compressed := gzip.NewWriter(w)
	defer compressed.Close()

	compressed.Comment = fmt.Sprintf("Generated the %s from %s", time.Now().Format(time.DateTime), baseURL)
	compressed.Name = filename
	compressed.ModTime = time.Now().UTC()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	if _, err := io.Copy(compressed, reader); err != nil {
		slog.ErrorContext(ctx, "could not write snapshot", slog.Any("error", errors.WithStack(err)))
		return
	}
}

const maxBackupBodySize = 1e+9 // 1Gb

func (h *Handler) handleRestoreBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, maxBackupBodySize)
	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		slog.ErrorContext(ctx, "could not parse multipart form", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	file, _, err := r.FormFile("file")
	if err != nil {
		slog.ErrorContext(ctx, "could not read form file", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	slog.DebugContext(ctx, "restoring backup")

	decompressed, err := gzip.NewReader(file)
	if err != nil {
		slog.ErrorContext(ctx, "could not decompress backup", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := h.documentManager.RestoreBackup(ctx, decompressed); err != nil {
		slog.ErrorContext(ctx, "could not restore backup", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Error(w, http.StatusText(http.StatusNoContent), http.StatusNoContent)
}
