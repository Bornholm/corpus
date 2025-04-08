package service

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/internal/markdown"
	"github.com/bornholm/corpus/internal/workflow"
	"github.com/pkg/errors"
)

type DocumentManagerOptions struct {
	MaxWordPerSection int
	FileConverter     port.FileConverter
}

type DocumentManagerOptionFunc func(opts *DocumentManagerOptions)

func WithDocumentManagerFileConverter(fileConverter port.FileConverter) DocumentManagerOptionFunc {
	return func(opts *DocumentManagerOptions) {
		opts.FileConverter = fileConverter
	}
}

func NewDocumentManagerOptions(funcs ...DocumentManagerOptionFunc) *DocumentManagerOptions {
	opts := &DocumentManagerOptions{
		MaxWordPerSection: 250,
	}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

type DocumentManager struct {
	maxWordPerSection int
	fileConverter     port.FileConverter
	port.Store
	port.TaskManager
	index port.Index
}

type DocumentManagerSearchOptions struct {
	MaxResults int
	// Names of the collection the query will be restricted to
	Collections []string
}

type DocumentManagerSearchOptionFunc func(opts *DocumentManagerSearchOptions)

func NewDocumentManagerSearchOptions(funcs ...DocumentManagerSearchOptionFunc) *DocumentManagerSearchOptions {
	opts := &DocumentManagerSearchOptions{
		MaxResults:  5,
		Collections: make([]string, 0),
	}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

func WithDocumentManagerSearchMaxResults(max int) DocumentManagerSearchOptionFunc {
	return func(opts *DocumentManagerSearchOptions) {
		opts.MaxResults = max
	}
}

func WithDocumentManagerSearchCollections(collections ...string) DocumentManagerSearchOptionFunc {
	return func(opts *DocumentManagerSearchOptions) {
		opts.Collections = collections
	}
}

func (m *DocumentManager) Search(ctx context.Context, query string, funcs ...DocumentManagerSearchOptionFunc) ([]*port.IndexSearchResult, error) {
	opts := NewDocumentManagerSearchOptions(funcs...)

	collections := make([]model.CollectionID, 0)
	for _, c := range opts.Collections {
		coll, err := m.Store.GetCollectionByName(ctx, c)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		collections = append(collections, coll.ID())
	}

	searchResults, err := m.index.Search(ctx, query, port.IndexSearchOptions{
		MaxResults:  opts.MaxResults,
		Collections: collections,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return searchResults, nil
}

type DocumentManagerIndexFileOptions struct {
	Source *url.URL
	// Names of the collection to associate with the document
	Collections []string
}

type DocumentManagerIndexFileOptionFunc func(opts *DocumentManagerIndexFileOptions)

func WithDocumentManagerIndexFileCollections(collections ...string) DocumentManagerIndexFileOptionFunc {
	return func(opts *DocumentManagerIndexFileOptions) {
		opts.Collections = collections
	}
}

func WithDocumentManagerIndexFileSource(source *url.URL) DocumentManagerIndexFileOptionFunc {
	return func(opts *DocumentManagerIndexFileOptions) {
		opts.Source = source
	}
}

func NewDocumentManagerIndexFileOptions(funcs ...DocumentManagerIndexFileOptionFunc) *DocumentManagerIndexFileOptions {
	opts := &DocumentManagerIndexFileOptions{}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

func (m *DocumentManager) SupportedExtensions() []string {
	return m.fileConverter.SupportedExtensions()
}

var ErrNotSupported = errors.New("not supported")

func (m *DocumentManager) IndexFile(ctx context.Context, filename string, r io.Reader, funcs ...DocumentManagerIndexFileOptionFunc) (port.TaskID, error) {
	opts := NewDocumentManagerIndexFileOptions(funcs...)

	tempDir, err := os.MkdirTemp("", "corpus-*")
	if err != nil {
		return "", errors.WithStack(err)
	}

	path := filepath.Join(tempDir, "document")

	file, err := os.Create(path)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if _, err := io.Copy(file, r); err != nil {
		return "", errors.WithStack(err)
	}

	taskID := port.NewTaskID()

	indexFileTask := &indexFileTask{
		id:           taskID,
		path:         path,
		originalName: filename,
		opts:         opts,
	}

	taskCtx := log.WithAttrs(context.Background(), slog.String("filename", filename))

	if err := m.TaskManager.Schedule(taskCtx, indexFileTask); err != nil {
		return "", errors.WithStack(err)
	}

	return taskID, nil
}

func (m *DocumentManager) handleIndexFileTask(ctx context.Context, task port.Task, progress chan float64) error {
	indexFileTask, ok := task.(*indexFileTask)
	if !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	var (
		document *markdown.Document
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
				if ext == ".md" || m.fileConverter == nil {
					reader = file
					progress <- 25
					return nil
				}

				supportedExtensions := m.fileConverter.SupportedExtensions()

				if supported := slices.Contains(supportedExtensions, ext); !supported {
					return errors.Wrapf(ErrNotSupported, "file extension '%s' is not supported by the file converter", ext)
				}

				readCloser, err := m.fileConverter.Convert(ctx, indexFileTask.originalName, file)
				if err != nil {
					return errors.WithStack(err)
				}

				reader = readCloser

				progress <- 25

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

				doc, err := markdown.Parse(data, markdown.WithMaxWordPerSection(m.maxWordPerSection))
				if err != nil {
					return errors.Wrap(err, "could not build document")
				}

				if indexFileTask.opts.Source != nil {
					doc.SetSource(indexFileTask.opts.Source)
				}

				if doc.Source() == nil {
					return errors.New("document source missing")
				}

				for _, name := range indexFileTask.opts.Collections {
					coll, err := m.Store.GetCollectionByName(ctx, name)
					if err != nil {
						if !errors.Is(err, port.ErrNotFound) {
							return errors.Wrapf(err, "could not find collection with name '%s'", name)
						}

						coll, err = m.Store.CreateCollection(ctx, name)
						if err != nil {
							return errors.WithStack(err)
						}
					}

					doc.AddCollection(coll)
				}

				document = doc

				progress <- 50

				return nil
			},
			nil,
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				if err := m.SaveDocument(ctx, document); err != nil {
					return errors.WithStack(err)
				}

				progress <- 60

				return nil
			},
			func(ctx context.Context) error {
				if err := m.DeleteDocumentBySource(ctx, document.Source()); err != nil {
					return errors.WithStack(err)
				}

				return nil
			},
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				if err := m.index.Index(ctx, document); err != nil {
					return errors.WithStack(err)
				}

				progress <- 90

				return nil
			},
			func(ctx context.Context) error {
				if err := m.index.DeleteBySource(ctx, document.Source()); err != nil {
					return errors.WithStack(err)
				}

				return nil
			},
		),
	)
	if err := wf.Execute(ctx); err != nil {
		return errors.WithStack(err)
	}

	progress <- 100

	return nil
}

func NewDocumentManager(store port.Store, index port.Index, taskManager port.TaskManager, funcs ...DocumentManagerOptionFunc) *DocumentManager {
	opts := NewDocumentManagerOptions(funcs...)

	documentManager := &DocumentManager{
		maxWordPerSection: opts.MaxWordPerSection,
		Store:             store,
		TaskManager:       taskManager,
		index:             index,
		fileConverter:     opts.FileConverter,
	}

	taskManager.Register(indexFileTaskType, port.TaskHandlerFunc(documentManager.handleIndexFileTask))

	return documentManager
}

const indexFileTaskType port.TaskType = "indexFile"

type indexFileTask struct {
	id           port.TaskID
	path         string
	originalName string
	opts         *DocumentManagerIndexFileOptions
}

// ID implements port.Task.
func (i *indexFileTask) ID() port.TaskID {
	return i.id
}

// Type implements port.Task.
func (i *indexFileTask) Type() port.TaskType {
	return indexFileTaskType
}

var _ port.Task = &indexFileTask{}
