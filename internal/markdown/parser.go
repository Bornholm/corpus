package markdown

import (
	"bytes"
	"net/url"
	"slices"

	"github.com/Bornholm/amatl/pkg/markdown/renderer/markdown"
	"github.com/Bornholm/amatl/pkg/markdown/renderer/markdown/node"
	"github.com/bornholm/corpus/internal/core/model"
	corpusText "github.com/bornholm/corpus/internal/text"
	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type Options struct {
	MaxWordPerSection int
}

type OptionFunc func(opts *Options)

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		MaxWordPerSection: 250,
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func WithMaxWordPerSection(maxWord int) OptionFunc {
	return func(opts *Options) {
		opts.MaxWordPerSection = maxWord
	}
}

func Parse(data []byte, funcs ...OptionFunc) (*Document, error) {
	opts := NewOptions(funcs...)
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			meta.Meta,
		),
		goldmark.WithRenderer(markdown.NewRenderer()),
		goldmark.WithRendererOptions(
			markdown.WithNodeRenderers(node.Renderers()),
		),
	)

	context := parser.NewContext()
	root := md.Parser().Parse(text.NewReader(data), parser.WithContext(context))

	document := &Document{
		id:          model.NewDocumentID(),
		collections: make([]model.Collection, 0),
	}

	current := &Section{
		document: document,
		level:    0,
		id:       model.NewSectionID(),
		sections: make([]*Section, 0),
	}

	current.branch = []model.SectionID{current.id}

	document.sections = []*Section{current}

	metadata := meta.Get(context)
	if rawSource, exists := metadata["source"]; exists {
		source, err := url.Parse(rawSource.(string))
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse metadata source url '%v'", rawSource)
		}

		document.source = source
	}

	renderer := md.Renderer()

	var buff bytes.Buffer

	split := false

	err := ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			if !split && current.parent != nil && current.content == current.parent.content {
				current.parent.sections = slices.DeleteFunc(current.parent.sections, func(s *Section) bool { return s == current })
			}

			return ast.WalkContinue, nil
		}

		buff.Reset()

		previous := current

		switch el := n.(type) {
		case *ast.Text:
			// No op
		case *ast.Document:
			// No op
		case *ast.Heading:
			split = false

			if uint(el.Level) > current.level {
				current = &Section{
					id:       model.NewSectionID(),
					document: document,
					level:    uint(el.Level),
					sections: make([]*Section, 0),
				}

				current.parent = findClosestAncestor(previous, uint(el.Level))

				if current.parent != nil {
					current.parent.sections = append(current.parent.sections, current)
					current.branch = append(current.parent.branch, current.id)
				} else {
					document.sections = append(document.sections, current)
					current.branch = []model.SectionID{current.id}
				}
			} else {
				current = &Section{
					document: document,
					id:       model.NewSectionID(),
					level:    uint(el.Level),
					sections: make([]*Section, 0),
				}

				current.parent = findClosestAncestor(previous, uint(el.Level))

				if current.parent != nil {
					current.parent.sections = append(current.parent.sections, current)
					current.branch = append(current.parent.branch, current.id)
				} else {
					document.sections = append(document.sections, current)
					current.branch = []model.SectionID{current.id}
				}
			}

			if err := renderer.Render(&buff, data, n); err != nil {
				return ast.WalkStop, errors.WithStack(err)
			}

			current.Append(buff.String())
		default:
			if err := renderer.Render(&buff, data, n); err != nil {
				return ast.WalkStop, errors.WithStack(err)
			}

			current.Append(buff.String())

			totalWords := len(corpusText.SplitByWords(current.content))

			if totalWords < opts.MaxWordPerSection {
				return ast.WalkContinue, nil
			}

			current = &Section{
				document: document,
				id:       model.NewSectionID(),
				level:    uint(current.level + 1),
				sections: make([]*Section, 0),
				parent:   previous,
			}

			current.branch = append(previous.branch, current.id)

			if split {
				current.level = previous.level
				current.parent = previous.parent
				current.branch = append(previous.parent.branch, current.id)
				previous.parent.sections = append(previous.parent.sections, current)
			} else {
				current.content = previous.content
				current.parent.sections = append(current.parent.sections, current)
			}

			split = true
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return document, nil
}

type Document struct {
	id          model.DocumentID
	source      *url.URL
	collections []model.Collection
	sections    []*Section
}

type Collection struct {
	id          model.CollectionID
	name        string
	description string
}

// Name implements model.Collection.
func (c *Collection) Name() string {
	return c.name
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

func (d *Document) SetSource(source *url.URL) {
	d.source = source
}

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

var _ model.Document = &Document{}

type Section struct {
	id       model.SectionID
	branch   []model.SectionID
	content  string
	level    uint
	document *Document
	parent   *Section
	sections []*Section
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

func (s *Section) Append(txt string) {
	s.content += txt
	if s.parent != nil {
		s.parent.Append(txt)
	}
}

// Content implements model.Section.
func (s *Section) Content() string {
	return s.content
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

func findClosestAncestor(from *Section, level uint) *Section {
	if from == nil {
		return nil
	}

	if from.level < level {
		return from
	}

	return findClosestAncestor(from.parent, level)
}
