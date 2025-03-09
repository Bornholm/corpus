package adapter

import (
	"database/sql/driver"
	"net/url"
	"strings"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

type Document struct {
	ID        string `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Source    string     `gorm:"unique;not null"`
	Sections  []*Section `gorm:"constraint:OnDelete:CASCADE"`
}

type wrappedDocument struct {
	d *Document
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
		ID:       string(d.ID()),
		Source:   d.Source().String(),
		Sections: make([]*Section, 0, len(d.Sections())),
	}

	for _, s := range d.Sections() {
		document.Sections = append(document.Sections, fromSection(document, s))
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
