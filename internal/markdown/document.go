package markdown

import (
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

type Document struct {
	data        []byte
	id          model.DocumentID
	ownerID     model.UserID
	etag        string
	source      *url.URL
	collections []model.Collection
	sections    []*Section
}

// OwnerID implements [model.Document].
func (d *Document) OwnerID() model.UserID {
	return d.ownerID
}

func (d *Document) SetOwnerID(ownerID model.UserID) {
	d.ownerID = ownerID
}

// ETag implements model.Document.
func (d *Document) ETag() string {
	return d.etag
}

func (d *Document) SetETag(etag string) {
	d.etag = etag
}

// Chunk implements model.Document.
func (d *Document) Chunk(start int, end int) ([]byte, error) {
	if start < 0 || end > len(d.data) {
		return nil, errors.New("out of range")
	}

	return d.data[start:end], nil
}

// Content implements model.Document.
func (d *Document) Content() ([]byte, error) {
	return d.data, nil
}

type Collection struct {
	id          model.CollectionID
	ownerID     model.UserID
	label       string
	description string
}

// Label implements model.Collection.
func (c *Collection) Label() string {
	return c.label
}

// OwnerID implements model.Collection.
func (c *Collection) OwnerID() model.UserID {
	return c.ownerID
}

// Description implements model.Collection.
func (c *Collection) Description() string {
	return c.description
}

// ID implements model.Collection.
func (c *Collection) ID() model.CollectionID {
	return c.id
}

var _ model.Collection = &Collection{}

func (d *Document) AddCollection(coll model.Collection) {
	d.collections = append(d.collections, coll)
}

// Collections implements model.Document.
func (d *Document) Collections() []model.Collection {
	return d.collections
}

// ID implements model.Document.
func (d *Document) ID() model.DocumentID {
	return d.id
}

// Sections implements model.Document.
func (d *Document) Sections() []model.Section {
	sections := make([]model.Section, len(d.sections))
	for i, s := range d.sections {
		sections[i] = s
	}
	return sections
}

// Source implements model.Document.
func (d *Document) Source() *url.URL {
	return d.source
}

func (d *Document) SetSource(source *url.URL) {
	d.source = source
}

var _ model.Document = &Document{}

type Section struct {
	id       model.SectionID
	branch   []model.SectionID
	level    uint
	document *Document
	parent   *Section
	sections []*Section
	start    int
	end      int
}

// Content implements model.Section.
func (s *Section) Content() ([]byte, error) {
	chunk, err := s.Document().Chunk(s.start, s.end)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	trimmed, err := Trim(chunk)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return trimmed, nil
}

// End implements model.Section.
func (s *Section) End() int {
	return s.end
}

// Start implements model.Section.
func (s *Section) Start() int {
	return s.start
}

// Branch implements model.Section.
func (s *Section) Branch() []model.SectionID {
	return s.branch
}

// Level implements model.Section.
func (s *Section) Level() uint {
	return uint(s.level)
}

// ID implements model.Section.
func (s *Section) ID() model.SectionID {
	return s.id
}

func (s *Section) AppendRange(end int) {
	if end > s.end {
		s.end = end
	}
	if s.parent != nil {
		s.parent.AppendRange(end)
	}
}

// Document implements model.Section.
func (s *Section) Document() model.Document {
	return s.document
}

// Parent implements model.Section.
func (s *Section) Parent() model.Section {
	return s.parent
}

// Sections implements model.Section.
func (s *Section) Sections() []model.Section {
	sections := make([]model.Section, len(s.sections))
	for i, s := range s.sections {
		sections[i] = s
	}
	return sections
}

var _ model.Section = &Section{}
