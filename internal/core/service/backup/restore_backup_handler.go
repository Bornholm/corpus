package backup

import (
	"context"
	"log/slog"
	"os"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type RestoreBackupHandler struct {
	documentStore port.DocumentStore
	backupManager *Manager
}

// Handle implements [port.TaskHandler].
func (h *RestoreBackupHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	restoreBackupTask, ok := task.(*RestoreBackupTask)
	if !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	defer func() {
		events <- port.NewTaskEvent(port.WithTaskProgress(1))
	}()

	defer func() {
		if err := os.Remove(restoreBackupTask.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.ErrorContext(ctx, "could not remove file", slog.Any("error", errors.WithStack(err)))
		}
	}()

	backupFile, err := os.Open(restoreBackupTask.path)
	if err != nil {
		return errors.Wrap(err, "could not open backup file")
	}

	defer backupFile.Close()

	snapshotable := h.backupManager.GetCompositeSnapshotable()

	slog.InfoContext(ctx, "restoring snapshot")
	events <- port.NewTaskEvent(port.WithTaskMessage("restoring snapshots"))

	if err := snapshotable.RestoreSnapshot(ctx, backupFile); err != nil {
		return errors.WithStack(err)
	}

	restorable := h.backupManager.GetRestorables()

	events <- port.NewTaskEvent(port.WithTaskMessage("restoring documents"), port.WithTaskProgress(0.5))

	page := 0
	limit := 100
	for {
		slog.DebugContext(ctx, "querying documents to restore", slog.Int("page", page), slog.Int("limit", limit))

		documents, total, err := h.documentStore.QueryDocuments(ctx, port.QueryDocumentsOptions{Page: &page, Limit: &limit})
		if err != nil {
			return errors.WithStack(err)
		}

		progress := float64(page*limit) / float64(total)
		events <- port.NewTaskEvent(port.WithTaskProgress(float32((0.5 + progress/2))))

		if len(documents) == 0 {
			return nil
		}

		slog.InfoContext(ctx, "restoring documents", slog.Int64("totalDocuments", total), slog.Int("progressPercent", int(progress*100)))

		for _, r := range restorable {
			docs := make([]model.Document, len(documents))
			for i, d := range documents {
				docs[i] = d
			}

			if err := r.RestoreDocuments(ctx, docs); err != nil {
				return errors.WithStack(err)
			}
		}

		page++
	}
}

func NewRestoreBackupHandler(documentStore port.DocumentStore, backupManager *Manager) *RestoreBackupHandler {
	return &RestoreBackupHandler{
		documentStore: documentStore,
		backupManager: backupManager,
	}
}

var _ port.TaskHandler = &RestoreBackupHandler{}
