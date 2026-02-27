package testsuite

import (
	"context"
	"embed"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/markdown"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
)

//go:embed testdata/**/*
var testdata embed.FS

func TestIndex(t *testing.T, factory func(t *testing.T) (port.Index, error)) {
	type testCase struct {
		Name string
		Run  func(t *testing.T, ctx context.Context, index port.Index) error
	}

	var testCases []testCase = []testCase{
		{
			Name: "SimpleQuery",
			Run: func(t *testing.T, ctx context.Context, index port.Index) error {
				if _, err := loadTestDocuments(t, index); err != nil {
					return errors.WithStack(err)
				}

				query := "De quand date la recette du boeuf bourguignon ?"

				t.Logf("executing query '%s'", query)

				results, err := index.Search(ctx, query, port.IndexSearchOptions{})
				if err != nil {
					return errors.WithStack(err)
				}

				t.Logf("results: %s", spew.Sdump(results))

				if len(results) == 0 {
					t.Fatalf("len(results): no results")
				}

				if e, g := "https://fr.wikipedia.org/wiki/B%C5%93uf_bourguignon", results[0].Source.String(); e != g {
					t.Errorf("results[0].Source.String(): expected %s, got %s", e, g)
				}

				return nil
			},
		},
		{
			Name: "FilterByCollection",
			Run: func(t *testing.T, ctx context.Context, index port.Index) error {
				collections, err := loadTestDocuments(t, index)
				if err != nil {
					return errors.WithStack(err)
				}

				programmingCollection, exists := collections["programming"]
				if !exists {
					return errors.New("could not find 'programming' collection")
				}

				query := "Par qui a été créé le langage Go ?"

				t.Logf("executing query '%s'", query)

				results, err := index.Search(ctx, query, port.IndexSearchOptions{
					Collections: []model.CollectionID{programmingCollection.ID()},
				})
				if err != nil {
					return errors.WithStack(err)
				}

				t.Logf("results: %s", spew.Sdump(results))

				if len(results) == 0 {
					t.Fatalf("len(results): no results")
				}

				if e, g := 2, len(results); e != g {
					t.Errorf("len(results): expected %d, got %d", e, g)
				}

				if e, g := "https://fr.wikipedia.org/wiki/Go_(langage)", results[0].Source.String(); e != g {
					t.Errorf("results[0].Source.String(): expected %s, got %s", e, g)
				}

				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			index, err := factory(t)
			if err != nil {
				t.Fatalf("could not create index: %+v", errors.WithStack(err))
			}

			if err := tc.Run(t, ctx, index); err != nil {
				t.Fatalf("could not run test: %+v", errors.WithStack(err))
			}
		})
	}
}

func loadTestDocuments(t *testing.T, index port.Index) (map[string]model.Collection, error) {
	ctx := context.TODO()

	jdoe := model.NewUserID()

	files, err := fs.Glob(testdata, "testdata/documents/*.md")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	t.Logf("loading %d documents", len(files))

	collections := map[string]model.Collection{}

	for _, f := range files {
		data, err := testdata.ReadFile(f)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		doc, err := markdown.Parse(data)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		filename := filepath.Base(f)
		collectionName, _, _ := strings.Cut(filename, "_")

		coll, exists := collections[collectionName]
		if !exists {
			coll = model.NewCollection(
				model.NewCollectionID(),
				jdoe,
				"",
				"",
			)
			collections[collectionName] = coll
		}

		doc.AddCollection(coll)

		t.Logf("indexing document %s within collections %v", doc.Source(), slices.Collect(func(yield func(string) bool) {
			for _, c := range doc.Collections() {
				if !yield(c.Label()) {
					return
				}
			}
		}))

		if err := index.Index(ctx, doc); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return collections, nil
}
