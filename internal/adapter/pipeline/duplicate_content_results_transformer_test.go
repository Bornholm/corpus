package pipeline

import (
	"context"
	"net/url"
	"slices"
	"testing"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

func TestBranchMergeResultsTransformer(t *testing.T) {
	transformer := NewDuplicateContentResultsTransformer(&dummyStore{
		sections: map[model.SectionID]model.Section{
			"parent1": &dummySection{
				id:     "parent1",
				branch: []model.SectionID{"parent1"},
			},
			"child1": &dummySection{
				id:     "child1",
				branch: []model.SectionID{"parent1", "child1"},
			},
			"child2": &dummySection{
				id:     "child2",
				branch: []model.SectionID{"parent1", "child2"},
			},
			"grandchild1": &dummySection{
				id:     "grandchild1",
				branch: []model.SectionID{"parent1", "child2", "grandchild1"},
			},
			"grandchild2": &dummySection{
				id:     "grandchild2",
				branch: []model.SectionID{"parent1", "child2", "grandchild2"},
			},
			"parent2": &dummySection{
				id:     "parent2",
				branch: []model.SectionID{"parent2"},
			},
			"child3": &dummySection{
				id:     "child3",
				branch: []model.SectionID{"parent2", "child3"},
			},
			"parent3": &dummySection{
				id:     "parent3",
				branch: []model.SectionID{"parent3"},
			},
		},
	})

	source, _ := url.Parse("https://example.net")

	results := []*port.IndexSearchResult{
		{
			Source: source,
			Sections: []model.SectionID{
				"parent1",
				"child1",
				"child2",
				"grandchild1",
				"grandchild2",
				"parent2",
				"child3",
				"parent3",
			},
		},
	}

	transformed, err := transformer.TransformResults(context.Background(), "test", results)
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	expected := []model.SectionID{"child1", "grandchild1", "grandchild2", "child3", "parent3"}

	for _, e := range expected {
		if !slices.Contains(transformed[0].Sections, e) {
			t.Errorf("transformed[0].Sections: expected '%s' not found", e)
		}
	}

	if e, g := len(expected), len(transformed[0].Sections); e != g {
		t.Errorf("len(transformed[0].Sections): expected '%d', got '%d'", e, g)
	}
}

type dummyStore struct {
	sections map[model.SectionID]model.Section
}

// DeleteDocumentByID implements port.Store.
func (d *dummyStore) DeleteDocumentByID(ctx context.Context, id model.DocumentID) error {
	panic("unimplemented")
}

// SectionExists implements port.Store.
func (d *dummyStore) SectionExists(ctx context.Context, id model.SectionID) (bool, error) {
	panic("unimplemented")
}

// GetDocumentByID implements port.Store.
func (d *dummyStore) GetDocumentByID(ctx context.Context, id model.DocumentID) (model.Document, error) {
	panic("unimplemented")
}

// CountDocuments implements port.Store.
func (d *dummyStore) CountDocuments(ctx context.Context) (int64, error) {
	panic("unimplemented")
}

// CreateCollection implements port.Store.
func (d *dummyStore) CreateCollection(ctx context.Context, name string) (model.Collection, error) {
	panic("unimplemented")
}

// DeleteDocumentBySource implements port.Store.
func (d *dummyStore) DeleteDocumentBySource(ctx context.Context, source *url.URL) error {
	panic("unimplemented")
}

// GetCollectionByName implements port.Store.
func (d *dummyStore) GetCollectionByName(ctx context.Context, name string) (model.Collection, error) {
	panic("unimplemented")
}

// GetCollectionStats implements port.Store.
func (d *dummyStore) GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error) {
	panic("unimplemented")
}

// GetSectionByID implements port.Store.
func (d *dummyStore) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
	return d.sections[id], nil
}

// QueryCollections implements port.Store.
func (d *dummyStore) QueryCollections(ctx context.Context, opts port.QueryCollectionsOptions) ([]model.Collection, error) {
	panic("unimplemented")
}

// QueryDocuments implements port.Store.
func (d *dummyStore) QueryDocuments(ctx context.Context, opts port.QueryDocumentsOptions) ([]model.Document, int64, error) {
	panic("unimplemented")
}

// SaveDocument implements port.Store.
func (d *dummyStore) SaveDocuments(ctx context.Context, documents ...model.Document) error {
	panic("unimplemented")
}

// UpdateCollection implements port.Store.
func (d *dummyStore) UpdateCollection(ctx context.Context, id model.CollectionID, updates port.CollectionUpdates) (model.Collection, error) {
	panic("unimplemented")
}

var _ port.Store = &dummyStore{}

type dummySection struct {
	id     model.SectionID
	branch []model.SectionID
	parent model.Section
}

// Branch implements model.Section.
func (d *dummySection) Branch() []model.SectionID {
	return d.branch
}

// Content implements model.Section.
func (d *dummySection) Content() ([]byte, error) {
	return []byte{}, nil
}

// Document implements model.Section.
func (d *dummySection) Document() model.Document {
	panic("unimplemented")
}

// End implements model.Section.
func (d *dummySection) End() int {
	return 1
}

// ID implements model.Section.
func (d *dummySection) ID() model.SectionID {
	return d.id
}

// Level implements model.Section.
func (d *dummySection) Level() uint {
	panic("unimplemented")
}

// Parent implements model.Section.
func (d *dummySection) Parent() model.Section {
	return d.parent
}

// Sections implements model.Section.
func (d *dummySection) Sections() []model.Section {
	return []model.Section{}
}

// Start implements model.Section.
func (d *dummySection) Start() int {
	return 0
}

var _ model.Section = &dummySection{}
