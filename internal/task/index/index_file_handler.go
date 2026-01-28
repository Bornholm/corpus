package index

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/markdown"
	"github.com/bornholm/corpus/internal/workflow"
	"github.com/pkg/errors"
)

type IndexFileHandler struct {
	userStore         port.UserStore
	documentStore     port.DocumentStore
	fileConverter     port.FileConverter
	index             port.Index
	maxWordPerSection int
}

func NewIndexFileHandler(userStore port.UserStore, documentStore port.DocumentStore, fileConverter port.FileConverter, index port.Index, maxWordPerSection int) *IndexFileHandler {
	return &IndexFileHandler{
		userStore:         userStore,
		documentStore:     documentStore,
		fileConverter:     fileConverter,
		index:             index,
		maxWordPerSection: maxWordPerSection,
	}
}

// Handle implements [port.TaskHandler].
func (h *IndexFileHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	indexFileTask, ok := task.(*IndexFileTask)
	if !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	defer func() {
		if err := os.Remove(indexFileTask.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.ErrorContext(ctx, "could not remove file", slog.Any("error", errors.WithStack(err)))
		}
	}()

	var (
		document model.OwnedDocument
	)

	var reader io.ReadCloser

	wf := workflow.New(
		workflow.StepFunc(
			func(ctx context.Context) error {
				file, err := os.Open(indexFileTask.path)
				if err != nil {
					return errors.WithStack(err)
				}

				ext := filepath.Ext(indexFileTask.originalName)
				if ext == ".md" || h.fileConverter == nil {
					reader = file
					events <- port.NewTaskEvent(port.WithTaskProgress(0.05))
					return nil
				}

				supportedExtensions := h.fileConverter.SupportedExtensions()

				if supported := slices.Contains(supportedExtensions, ext); !supported {
					return errors.Wrapf(port.ErrNotSupported, "file extension '%s' is not supported by the file converter", ext)
				}

				events <- port.NewTaskEvent(port.WithTaskMessage("converting document"), port.WithTaskProgress(0.01))

				readCloser, err := h.fileConverter.Convert(ctx, indexFileTask.originalName, file)
				if err != nil {
					return errors.WithStack(err)
				}

				reader = readCloser

				events <- port.NewTaskEvent(port.WithTaskProgress(0.05))

				return nil
			},
			nil,
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				defer reader.Close()

				data, err := io.ReadAll(reader)
				if err != nil {
					return errors.WithStack(err)
				}

				events <- port.NewTaskEvent(port.WithTaskMessage("parsing document"))

				doc, err := markdown.Parse(
					data,
					markdown.WithMaxWordPerSection(h.maxWordPerSection),
				)
				if err != nil {
					return errors.Wrap(err, "could not parse document")
				}

				events <- port.NewTaskEvent(port.WithTaskMessage("document parsed"))

				if indexFileTask.source != nil {
					doc.SetSource(indexFileTask.source)
				}

				if doc.Source() == nil {
					return errors.New("document source missing")
				}

				if indexFileTask.etag != "" {
					doc.SetETag(indexFileTask.etag)
				}

				writableCollections, _, err := h.documentStore.QueryUserWritableCollections(ctx, indexFileTask.Owner().ID(), port.QueryCollectionsOptions{})
				if err != nil {
					return errors.Wrap(err, "could not retrieve user writable collections")
				}

				if len(indexFileTask.collections) == 0 {
					return errors.New("no specified target collections")
				}

				for _, collectionID := range indexFileTask.collections {
					isWritable := slices.ContainsFunc(writableCollections, func(c model.PersistedCollection) bool {
						return collectionID == c.ID()
					})

					if !isWritable {
						return errors.Errorf("collection '%s' is not writable to the user '%s'", collectionID, indexFileTask.Owner().ID())
					}

					coll, err := h.documentStore.GetCollectionByID(ctx, collectionID)
					if err != nil {
						return errors.WithStack(err)
					}

					doc.AddCollection(coll)
				}

				user, err := h.userStore.GetUserByID(ctx, indexFileTask.Owner().ID())
				if err != nil {
					return errors.Wrap(err, "could not retrieve task owner")
				}

				document = model.AsOwnedDocument(doc, user)

				events <- port.NewTaskEvent(port.WithTaskProgress(0.1))

				return nil
			},
			nil,
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				events <- port.NewTaskEvent(port.WithTaskMessage("saving document"))

				if err := h.documentStore.SaveDocuments(ctx, document); err != nil {
					return errors.WithStack(err)
				}

				events <- port.NewTaskEvent(port.WithTaskProgress(0.2), port.WithTaskMessage("document saved"))

				return nil
			},
			func(ctx context.Context) error {
				if err := h.documentStore.DeleteDocumentBySource(ctx, indexFileTask.Owner().ID(), document.Source()); err != nil {
					return errors.WithStack(err)
				}

				return nil
			},
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				onProgress := func(p float32) {
					events <- port.NewTaskEvent(port.WithTaskProgress(0.2 + (0.7 * p)))
				}

				events <- port.NewTaskEvent(port.WithTaskMessage("indexing document"))

				if err := h.index.Index(ctx, document, port.WithIndexOnProgress(onProgress)); err != nil {
					return errors.WithStack(err)
				}

				events <- port.NewTaskEvent(port.WithTaskMessage("document indexed"))

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

	events <- port.NewTaskEvent(port.WithTaskProgress(1), port.WithTaskMessage("done"))

	return nil
}

var _ port.TaskHandler = &IndexFileHandler{}
