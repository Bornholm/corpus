package gorm

import (
	"database/sql/driver"
	"net/url"
	"strings"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

type Document struct {
	ID          string `gorm:"primarykey"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Source      string        `gorm:"unique;not null;index"`
	Sections    []*Section    `gorm:"constraint:OnDelete:CASCADE"`
	Collections []*Collection `gorm:"many2many:documents_collections;"`
}

type wrappedDocument struct {
	d *Document
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

func fromDocument(d model.Document) *Document {
	document := &Document{
		ID:          string(d.ID()),
		Source:      d.Source().String(),
		Collections: make([]*Collection, 0, len(d.Collections())),
		Sections:    make([]*Section, 0, len(d.Sections())),
	}

	for _, s := range d.Sections() {
		document.Sections = append(document.Sections, fromSection(document, s))
	}

	for _, c := range d.Collections() {
		document.Collections = append(document.Collections, fromCollection(c))
	}

	return document
}

type Section struct {
	ID string `gorm:"primarykey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Document   *Document
	DocumentID string `gorm:"primaryKey;autoIncrement:false"`

	Parent           *Section
	ParentID         *string
	ParentDocumentID *string

	Sections []*Section `gorm:"foreignKey:ParentID,ParentDocumentID"`

	Branch *Branch
	Level  uint

	Content string
}

type wrappedSection struct {
	s *Section
}

// Branch implements model.Section.
func (w *wrappedSection) Branch() []model.SectionID {
	return *w.s.Branch
}

// Level implements model.Section.
func (w *wrappedSection) Level() uint {
	return w.s.Level
}

// Content implements model.Section.
func (w *wrappedSection) Content() string {
	return w.s.Content
}

// Document implements model.Section.
func (w *wrappedSection) Document() model.Document {
	return &wrappedDocument{w.s.Document}
}

// ID implements model.Section.
func (w *wrappedSection) ID() model.SectionID {
	return model.SectionID(w.s.ID)
}

// Parent implements model.Section.
func (w *wrappedSection) Parent() model.Section {
	if w.s.Parent == nil {
		return nil
	}

	return &wrappedSection{s: w.s.Parent}
}

// Sections implements model.Section.
func (w *wrappedSection) Sections() []model.Section {
	sections := make([]model.Section, 0, len(w.s.Sections))
	for _, s := range w.s.Sections {
		sections = append(sections, &wrappedSection{s})
	}
	return sections
}

var _ model.Section = &wrappedSection{}

func fromSection(d *Document, s model.Section) *Section {
	branch := Branch(s.Branch())
	section := &Section{
		ID:         string(s.ID()),
		Document:   d,
		DocumentID: d.ID,
		Branch:     &branch,
		Sections:   make([]*Section, 0, len(s.Sections())),
		Content:    s.Content(),
	}

	for _, s := range s.Sections() {
		ss := fromSection(d, s)
		ss.Parent = section
		section.Sections = append(section.Sections, ss)
	}

	return section
}

type Branch []model.SectionID

func (b *Branch) Scan(value interface{}) error {
	text, ok := value.(string)
	if !ok {
		return errors.Errorf("unexpected type '%T'", value)
	}

	parts := strings.Split(text, ".")

	bb := make([]model.SectionID, len(parts))
	for i, p := range parts {
		bb[i] = model.SectionID(p)
	}

	*b = bb

	return nil
}

// Value return json value, implement driver.Valuer interface
func (b *Branch) Value() (driver.Value, error) {
	parts := make([]string, len(*b))
	for i, p := range *b {
		parts[i] = string(p)
	}

	return strings.Join(parts, "."), nil
}

type Collection struct {
	ID string `gorm:"primarykey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Name        string `gorm:"unique"`
	Description string
}

type wrappedCollection struct {
	c *Collection
}

// Description implements model.Collection.
func (w *wrappedCollection) Description() string {
	return w.c.Description
}

// ID implements model.Collection.
func (w *wrappedCollection) ID() model.CollectionID {
	return model.CollectionID(w.c.ID)
}

// Name implements model.Collection.
func (w *wrappedCollection) Name() string {
	return w.c.Name
}

var _ model.Collection = &wrappedCollection{}

func fromCollection(c model.Collection) *Collection {
	collection := &Collection{
		ID:          string(c.ID()),
		Name:        c.Name(),
		Description: c.Description(),
	}

	return collection
}
