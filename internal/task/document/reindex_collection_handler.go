package document

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type ReindexCollectionHandler struct {
	documentStore     port.DocumentStore
	index             port.Index
	maxWordPerSection int
}

func NewReindexCollectionHandler(documentStore port.DocumentStore, index port.Index, maxWordPerSection int) *ReindexCollectionHandler {
	return &ReindexCollectionHandler{
		documentStore:     documentStore,
		index:             index,
		maxWordPerSection: maxWordPerSection,
	}
}

// Handle implements [port.TaskHandler].
func (h *ReindexCollectionHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	select {
	case <-ctx.Done():
		return errors.WithStack(ctx.Err())
	default:
	}

	reindexTask, ok := task.(*ReindexCollectionTask)
	if !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	events <- port.NewTaskEvent(port.WithTaskMessage("retrieving total documents in collection"))

	// Get all documents in the collection
	limit := 1
	page := 1
	var totalDocuments int64 = 0

	// First, get the total count
	_, total, err := h.documentStore.QueryDocumentsByCollectionID(ctx, reindexTask.collectionID, port.QueryDocumentsOptions{
		Limit:      &limit,
		Page:       &page,
		HeaderOnly: true,
	})
	if err != nil {
		return errors.Wrap(err, "could not query documents in collection")
	}
	totalDocuments = total

	if totalDocuments == 0 {
		events <- port.NewTaskEvent(port.WithTaskMessage("no document to reindex"), port.WithTaskProgress(1))
		return nil
	}

	// First, delete all existing index entries for this collection
	events <- port.NewTaskEvent(port.WithTaskMessage("deleting already existing index"))

	deletedCount := 0

	allDocumentsProcessed := false
	docPage := 1
	docLimit := 50

	for !allDocumentsProcessed {
		select {
		case <-ctx.Done():
			return errors.WithStack(ctx.Err())
		default:
		}

		documents, count, err := h.documentStore.QueryDocumentsByCollectionID(ctx, reindexTask.collectionID, port.QueryDocumentsOptions{
			Limit: &docLimit,
			Page:  &docPage,
		})
		if err != nil {
			return errors.Wrap(err, "could not query documents")
		}

		if len(documents) == 0 || count == 0 {
			allDocumentsProcessed = true
			break
		}

		// Delete index entries for these documents
		for _, doc := range documents {
			select {
			case <-ctx.Done():
				return errors.WithStack(ctx.Err())
			default:
			}

			source := doc.Source()
			if source != nil {
				if err := h.index.DeleteBySource(ctx, source); err != nil {
					slog.ErrorContext(ctx, "could not delete index entries for document", slog.Any("error", errors.WithStack(err)))
				}
				deletedCount++
			}
		}

		// Update progress
		progress := float32(docPage*docLimit) / float32(totalDocuments)
		if progress > 1 {
			progress = 1
		}
		events <- port.NewTaskEvent(port.WithTaskProgress(0.3*progress), port.WithTaskMessage("deleting old index entries"))

		if len(documents) < docLimit {
			allDocumentsProcessed = true
		}
		docPage++
	}

	slog.InfoContext(ctx, "deleted index entries", "count", deletedCount)

	// Now re-index all documents
	events <- port.NewTaskEvent(port.WithTaskMessage("reindexing documents"))

	allDocumentsProcessed = false
	docPage = 1

	for !allDocumentsProcessed {
		select {
		case <-ctx.Done():
			return errors.WithStack(ctx.Err())
		default:
		}

		documents, count, err := h.documentStore.QueryDocumentsByCollectionID(ctx, reindexTask.collectionID, port.QueryDocumentsOptions{
			Limit: &docLimit,
			Page:  &docPage,
		})
		if err != nil {
			return errors.Wrap(err, "could not query documents")
		}

		if len(documents) == 0 || count == 0 {
			allDocumentsProcessed = true
			break
		}

		for i, doc := range documents {
			select {
			case <-ctx.Done():
				return errors.WithStack(ctx.Err())
			default:
			}

			// Re-index the document (documents already have their collections from the store)
			if err := h.index.Index(ctx, doc); err != nil {
				slog.ErrorContext(ctx, "could not index document", slog.Any("error", errors.WithStack(err)))
			}

			// Update progress (30% to 100%)
			docProgress := float32((docPage-1)*docLimit+i+1) / float32(totalDocuments)
			progress := 0.3 + (0.7 * docProgress)
			events <- port.NewTaskEvent(port.WithTaskProgress(progress), port.WithTaskMessage("reindexing"))
		}

		if len(documents) < docLimit {
			allDocumentsProcessed = true
		}
		docPage++
	}

	events <- port.NewTaskEvent(port.WithTaskProgress(1), port.WithTaskMessage("reindexing finished"))

	return nil
}

var _ port.TaskHandler = &ReindexCollectionHandler{}
