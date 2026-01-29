package document

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type CleanupHandler struct {
	index         port.Index
	documentStore port.DocumentStore
}

func NewCleanupHandler(index port.Index, documentStore port.DocumentStore) *CleanupHandler {
	return &CleanupHandler{
		index:         index,
		documentStore: documentStore,
	}
}

// Handle implements [port.TaskHandler].
func (h *CleanupHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	cleanupTask, ok := task.(*CleanupTask)
	if !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	if err := h.cleanupOrphanedDocuments(ctx, cleanupTask); err != nil {
		return errors.Wrap(err, "could not cleanup orphaned sections")
	}

	if err := h.cleanupObsoleteSections(ctx, cleanupTask); err != nil {
		return errors.Wrap(err, "could not cleanup obsolete sections")
	}

	return nil
}

func (h *CleanupHandler) cleanupOrphanedDocuments(ctx context.Context, task *CleanupTask) error {
	slog.DebugContext(ctx, "checking orphaned document")

	count := 0
	batchSize := 5
	toDelete := make([]model.DocumentID, 0, batchSize)

	deleteCurrentBatch := func() {
		slog.InfoContext(ctx, "deleting orphaned documents", "document_ids", toDelete)

		if err := h.documentStore.DeleteDocumentByID(ctx, toDelete...); err != nil {
			slog.ErrorContext(ctx, "could not delete obsolete sections", slog.Any("error", errors.WithStack(err)))
		}

		slog.InfoContext(ctx, "orphaned documents deleted")

		count += len(toDelete)

		toDelete = make([]model.DocumentID, 0, batchSize)

		// Prevent overwhelming the database
		time.Sleep(250 * time.Millisecond)
	}

	limit := batchSize
	orphaned := true

	for {
		documents, _, err := h.documentStore.QueryUserWritableDocuments(ctx, task.owner.ID(), port.QueryDocumentsOptions{
			Limit:      &limit,
			HeaderOnly: true,
			Orphaned:   &orphaned,
		})
		if err != nil {
			return errors.Wrap(err, "could not query documents")
		}

		if len(documents) == 0 {
			break
		}

		documentIDs := make([]model.DocumentID, len(documents))
		for i, d := range documents {
			documentIDs[i] = d.ID()
		}

		toDelete = append(toDelete, documentIDs...)

		if len(toDelete) >= batchSize {
			deleteCurrentBatch()
		}
	}

	if len(toDelete) > 0 {
		deleteCurrentBatch()
	}

	slog.DebugContext(ctx, "orphaned documents deleted", slog.Int64("total", int64(count)))

	return nil
}

func (h *CleanupHandler) cleanupObsoleteSections(ctx context.Context, task *CleanupTask) error {
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

var _ port.TaskHandler = &CleanupHandler{}
