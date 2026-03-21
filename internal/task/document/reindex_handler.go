package document

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/pkg/errors"
)

type ReindexHandler struct {
	documentStore     port.DocumentStore
	index             port.Index
	maxWordPerSection int
}

func NewReindexHandler(documentStore port.DocumentStore, index port.Index, maxWordPerSection int) *ReindexHandler {
	return &ReindexHandler{
		documentStore:     documentStore,
		index:             index,
		maxWordPerSection: maxWordPerSection,
	}
}

// Handle implements [port.TaskHandler].
func (h *ReindexHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	slog.DebugContext(ctx, "reindex handler started")

	select {
	case <-ctx.Done():
		slog.DebugContext(ctx, "reindex handler context cancelled at start")
		return errors.WithStack(ctx.Err())
	default:
	}

	var collectionID model.CollectionID

	switch t := task.(type) {
	case *ReindexCollectionTask:
		collectionID = t.collectionID
	case *ReindexBleveTask:
		// No collection filter - reindex all
	default:
		return errors.Errorf("unexpected task type '%T'", task)
	}

	slog.DebugContext(ctx, "reindex handler sending first event")
	events <- port.NewTaskEvent(port.WithTaskMessage("retrieving total documents"))
	slog.DebugContext(ctx, "reindex handler first event sent")

	// Get all documents (optionally filtered by collection)
	limit := 1
	page := 1
	var totalDocuments int64 = 0

	var err error
	var total int64

	// First, get the total count
	if collectionID != "" {
		_, total, err = h.documentStore.QueryDocumentsByCollectionID(ctx, collectionID, port.QueryDocumentsOptions{
			Limit:      &limit,
			Page:       &page,
			HeaderOnly: true,
		})
	} else {
		_, total, err = h.documentStore.QueryDocuments(ctx, port.QueryDocumentsOptions{
			Limit:      &limit,
			Page:       &page,
			HeaderOnly: true,
		})
	}

	if err != nil {
		return errors.Wrap(err, "could not query documents")
	}
	totalDocuments = total

	if totalDocuments == 0 {
		events <- port.NewTaskEvent(port.WithTaskMessage("no document to reindex"), port.WithTaskProgress(1))
		return nil
	}

	// First, delete all existing index entries
	events <- port.NewTaskEvent(port.WithTaskMessage("deleting existing index entries"))

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

		var documents []model.PersistedDocument
		var count int64

		if collectionID != "" {
			documents, count, err = h.documentStore.QueryDocumentsByCollectionID(ctx, collectionID, port.QueryDocumentsOptions{
				Limit: &docLimit,
				Page:  &docPage,
			})
		} else {
			documents, count, err = h.documentStore.QueryDocuments(ctx, port.QueryDocumentsOptions{
				Limit: &docLimit,
				Page:  &docPage,
			})
		}

		if err != nil {
			return errors.Wrap(err, "could not query documents")
		}

		if len(documents) == 0 || count == 0 {
			allDocumentsProcessed = true
			break
		}

		events <- port.NewTaskEvent(port.WithTaskMessage(fmt.Sprintf("deleting documents (total: %d, batch: %d, batch size: %d)", count, docPage, docLimit)))

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

		var documents []model.PersistedDocument
		var count int64

		if collectionID != "" {
			documents, count, err = h.documentStore.QueryDocumentsByCollectionID(ctx, collectionID, port.QueryDocumentsOptions{
				Limit: &docLimit,
				Page:  &docPage,
			})
		} else {
			documents, count, err = h.documentStore.QueryDocuments(ctx, port.QueryDocumentsOptions{
				Limit: &docLimit,
				Page:  &docPage,
			})
		}

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

var _ port.TaskHandler = &ReindexHandler{}
