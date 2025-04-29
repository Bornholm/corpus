package gorm

import (
	"net/url"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

type Document struct {
	ID          string `gorm:"primaryKey;autoIncrement:false"`
	ETag        string `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Source      string        `gorm:"unique;not null;index"`
	Sections    []*Section    `gorm:"constraint:OnDelete:CASCADE"`
	Collections []*Collection `gorm:"many2many:documents_collections;"`
	Content     []byte
}

type wrappedDocument struct {
	d *Document
}

// ETag implements model.Document.
func (w *wrappedDocument) ETag() string {
	return w.d.ETag
}

// Chunk implements model.Document.
func (w *wrappedDocument) Chunk(start int, end int) ([]byte, error) {
	if start < 0 || end > len(w.d.Content) {
		return nil, errors.WithStack(model.ErrOutOfRange)
	}

	return w.d.Content[start:end], nil
}

// Content implements model.Document.
func (w *wrappedDocument) Content() ([]byte, error) {
	return w.d.Content, nil
}

// Collection implements model.Document.
func (w *wrappedDocument) Collections() []model.Collection {
	collections := make([]model.Collection, 0, len(w.d.Collections))
	for _, c := range w.d.Collections {
		collections = append(collections, &wrappedCollection{c})
	}
	return collections
}

// ID implements model.Document.
func (w *wrappedDocument) ID() model.DocumentID {
	return model.DocumentID(w.d.ID)
}

// Sections implements model.Document.
func (w *wrappedDocument) Sections() []model.Section {
	sections := make([]model.Section, 0, len(w.d.Sections))
	for _, s := range w.d.Sections {
		s.Document = w.d
		sections = append(sections, &wrappedSection{s})
	}
	return sections
}

// Source implements model.Document.
func (w *wrappedDocument) Source() *url.URL {
	url, err := url.Parse(w.d.Source)
	if err != nil {
		panic(errors.WithStack(err))
	}

	return url
}

var _ model.Document = &wrappedDocument{}

func fromDocument(d model.Document) (*Document, error) {
	content, err := d.Content()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	document := &Document{
		ID:          string(d.ID()),
		ETag:        d.ETag(),
		Source:      d.Source().String(),
		Collections: make([]*Collection, 0, len(d.Collections())),
		Sections:    make([]*Section, 0, len(d.Sections())),
		Content:     content,
	}

	for _, s := range d.Sections() {
		document.Sections = append(document.Sections, fromSection(document, s))
	}

	for _, c := range d.Collections() {
		document.Collections = append(document.Collections, fromCollection(c))
	}

	return document, nil
}
