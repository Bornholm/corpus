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

	transformed, err := transformer.TransformResults(context.Background(), "test", results, port.IndexSearchOptions{})
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

// DeleteCollection implements [port.DocumentStore].
func (d *dummyStore) DeleteCollection(ctx context.Context, id model.CollectionID) error {
	panic("unimplemented")
}

// CanReadCollection implements [port.DocumentStore].
func (d *dummyStore) CanReadCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error) {
	panic("unimplemented")
}

// CanReadDocument implements [port.DocumentStore].
func (d *dummyStore) CanReadDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error) {
	panic("unimplemented")
}

// CanWriteCollection implements [port.DocumentStore].
func (d *dummyStore) CanWriteCollection(ctx context.Context, userID model.UserID, collectionID model.CollectionID) (bool, error) {
	panic("unimplemented")
}

// CanWriteDocument implements [port.DocumentStore].
func (d *dummyStore) CanWriteDocument(ctx context.Context, userID model.UserID, documentID model.DocumentID) (bool, error) {
	panic("unimplemented")
}

// CountReadableDocuments implements [port.DocumentStore].
func (d *dummyStore) CountReadableDocuments(ctx context.Context, userID model.UserID) (int64, error) {
	panic("unimplemented")
}

// CreateCollection implements [port.DocumentStore].
func (d *dummyStore) CreateCollection(ctx context.Context, ownerID model.UserID, label string) (model.PersistedCollection, error) {
	panic("unimplemented")
}

// DeleteDocumentByID implements [port.DocumentStore].
func (d *dummyStore) DeleteDocumentByID(ctx context.Context, ids ...model.DocumentID) error {
	panic("unimplemented")
}

// DeleteDocumentBySource implements [port.DocumentStore].
func (d *dummyStore) DeleteDocumentBySource(ctx context.Context, ownerID model.UserID, source *url.URL) error {
	panic("unimplemented")
}

// GetCollectionByID implements [port.DocumentStore].
func (d *dummyStore) GetCollectionByID(ctx context.Context, id model.CollectionID, full bool) (model.PersistedCollection, error) {
	panic("unimplemented")
}

// GetCollectionStats implements [port.DocumentStore].
func (d *dummyStore) GetCollectionStats(ctx context.Context, id model.CollectionID) (*model.CollectionStats, error) {
	panic("unimplemented")
}

// GetDocumentByID implements [port.DocumentStore].
func (d *dummyStore) GetDocumentByID(ctx context.Context, id model.DocumentID) (model.PersistedDocument, error) {
	panic("unimplemented")
}

// GetSectionByID implements [port.DocumentStore].
func (d *dummyStore) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
	return d.sections[id], nil
}

// QueryCollections implements [port.DocumentStore].
func (d *dummyStore) QueryCollections(ctx context.Context, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, error) {
	panic("unimplemented")
}

// QueryDocuments implements [port.DocumentStore].
func (d *dummyStore) QueryDocuments(ctx context.Context, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	panic("unimplemented")
}

// QueryUserReadableCollections implements [port.DocumentStore].
func (d *dummyStore) QueryUserReadableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error) {
	panic("unimplemented")
}

// QueryUserReadableDocuments implements [port.DocumentStore].
func (d *dummyStore) QueryUserReadableDocuments(ctx context.Context, userID model.UserID, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	panic("unimplemented")
}

// QueryUserWritableCollections implements [port.DocumentStore].
func (d *dummyStore) QueryUserWritableCollections(ctx context.Context, userID model.UserID, opts port.QueryCollectionsOptions) ([]model.PersistedCollection, int64, error) {
	panic("unimplemented")
}

// QueryUserWritableDocuments implements [port.DocumentStore].
func (d *dummyStore) QueryUserWritableDocuments(ctx context.Context, userID model.UserID, opts port.QueryDocumentsOptions) ([]model.PersistedDocument, int64, error) {
	panic("unimplemented")
}

// SaveDocuments implements [port.DocumentStore].
func (d *dummyStore) SaveDocuments(ctx context.Context, documents ...model.OwnedDocument) error {
	panic("unimplemented")
}

// SectionExists implements [port.DocumentStore].
func (d *dummyStore) SectionExists(ctx context.Context, id model.SectionID) (bool, error) {
	panic("unimplemented")
}

// UpdateCollection implements [port.DocumentStore].
func (d *dummyStore) UpdateCollection(ctx context.Context, id model.CollectionID, updates port.CollectionUpdates) (model.PersistedCollection, error) {
	panic("unimplemented")
}

var _ port.DocumentStore = &dummyStore{}

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
