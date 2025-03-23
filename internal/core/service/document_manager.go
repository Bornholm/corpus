package service

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/markdown"
	"github.com/bornholm/corpus/internal/workflow"
	"github.com/pkg/errors"
)

type DocumentManagerOptions struct {
	FileConverter port.FileConverter
}

type DocumentManagerOptionFunc func(opts *DocumentManagerOptions)

func WithDocumentManagerFileConverter(fileConverter port.FileConverter) DocumentManagerOptionFunc {
	return func(opts *DocumentManagerOptions) {
		opts.FileConverter = fileConverter
	}
}

func NewDocumentManagerOptions(funcs ...DocumentManagerOptionFunc) *DocumentManagerOptions {
	opts := &DocumentManagerOptions{}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

type DocumentManager struct {
	fileConverter port.FileConverter
	port.Store
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

var ErrNotSupported = errors.New("not supported")

func (m *DocumentManager) IndexFile(ctx context.Context, filename string, r io.Reader, funcs ...DocumentManagerIndexFileOptionFunc) (model.Document, error) {
	opts := NewDocumentManagerIndexFileOptions(funcs...)

	var (
		document *markdown.Document
	)

	wf := workflow.New(
		workflow.StepFunc(
			func(ctx context.Context) error {
				ext := filepath.Ext(filename)
				if ext == ".md" || m.fileConverter == nil {
					return nil
				}

				supportedExtensions := m.fileConverter.SupportedExtensions()

				if supported := slices.Contains(supportedExtensions, ext); !supported {
					return errors.Wrapf(ErrNotSupported, "file extension '%s' is not supported by the file converter", ext)
				}

				readCloser, err := m.fileConverter.Convert(ctx, filename, r)
				if err != nil {
					return errors.WithStack(err)
				}

				r = readCloser

				return nil
			},
			nil,
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				if rc := r.(io.ReadCloser); rc != nil {
					defer rc.Close()
				}

				data, err := io.ReadAll(r)
				if err != nil {
					return errors.WithStack(err)
				}

				doc, err := markdown.Parse(data)
				if err != nil {
					return errors.Wrap(err, "could not build document")
				}

				if opts.Source != nil {
					doc.SetSource(opts.Source)
				}

				if doc.Source() == nil {
					return errors.New("document source missing")
				}

				for _, name := range opts.Collections {
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

				return nil
			},
			nil,
		),
		workflow.StepFunc(
			func(ctx context.Context) error {
				if err := m.SaveDocument(ctx, document); err != nil {
					return errors.WithStack(err)
				}

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
		return nil, errors.WithStack(err)
	}

	return document, nil
}

func NewDocumentManager(store port.Store, index port.Index, funcs ...DocumentManagerOptionFunc) *DocumentManager {
	opts := NewDocumentManagerOptions(funcs...)
	return &DocumentManager{
		Store:         store,
		index:         index,
		fileConverter: opts.FileConverter,
	}
}
