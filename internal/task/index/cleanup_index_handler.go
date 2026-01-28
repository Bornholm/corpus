package index

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type CleanupIndexHandler struct {
	index         port.Index
	documentStore port.DocumentStore
}

func NewCleanupIndexHandler(index port.Index, documentStore port.DocumentStore) *CleanupIndexHandler {
	return &CleanupIndexHandler{
		index:         index,
		documentStore: documentStore,
	}
}

// Handle implements [port.TaskHandler].
func (h *CleanupIndexHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	if _, ok := task.(*CleanupIndexTask); !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	slog.DebugContext(ctx, "checking obsolete sections")

	count := 0
	batchSize := 5000
	toDelete := make([]model.SectionID, 0, batchSize)

	deleteCurrentBatch := func() {
		slog.InfoContext(ctx, "deleting obsolete sections from index")

		if err := h.index.DeleteByID(ctx, toDelete...); err != nil {
			slog.ErrorContext(ctx, "could not delete obsolete sections", slog.Any("error", errors.WithStack(err)))
		}

		slog.InfoContext(ctx, "obsolete sections deleted")

		toDelete = make([]model.SectionID, 0, batchSize)
	}
	err := h.index.All(ctx, func(id model.SectionID) bool {
		count++
		exists, err := h.documentStore.SectionExists(ctx, id)
		if err != nil {
			slog.ErrorContext(ctx, "could not check if section exists", slog.Any("error", errors.WithStack(err)))
			return true
		}

		if exists {
			return true
		}

		toDelete = append(toDelete, id)

		if len(toDelete) >= batchSize {
			deleteCurrentBatch()
		}

		return true
	})
	if err != nil {
		return errors.WithStack(err)
	}

	if len(toDelete) > 0 {
		deleteCurrentBatch()
	}

	slog.DebugContext(ctx, "all sections checked", slog.Int64("total", int64(count)))

	return nil
}

var _ port.TaskHandler = &CleanupIndexHandler{}
