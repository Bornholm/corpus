package bleve

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/blevesearch/bleve/v2"
	bleveQuery "github.com/blevesearch/bleve/v2/search/query"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type Index struct {
	index bleve.Index
}

// DeleteBySource implements port.Index.
func (i *Index) DeleteBySource(ctx context.Context, source *url.URL) error {
	query := bleve.NewTermQuery(source.String())
	query.SetField("source")
	req := &bleve.SearchRequest{
		Query: query,
		Size:  100,
	}

	for {
		result, err := i.index.Search(req)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, r := range result.Hits {
			if !strings.HasPrefix(r.ID, source.String()) {
				continue
			}

			slog.DebugContext(ctx, "deleting resource", slog.String("source", source.String()), slog.String("id", r.ID))

			if err := i.index.Delete(r.ID); err != nil {
				return errors.WithStack(err)
			}
		}

		if result.Total <= uint64(req.Size) {
			break
		}
	}

	return nil
}

// Index implements port.Index.
func (i *Index) Index(ctx context.Context, document model.Document) error {
	source := document.Source()

	if err := i.DeleteBySource(ctx, source); err != nil {
		return errors.WithStack(err)
	}

	for _, s := range document.Sections() {
		if err := i.indexSection(ctx, s); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (i *Index) indexSection(ctx context.Context, section model.Section) error {
	for _, s := range section.Sections() {
		if err := i.indexSection(ctx, s); err != nil {
			return errors.WithStack(err)
		}
	}

	source := section.Document().Source()
	sectionID := source.JoinPath()
	sectionID.Fragment = string(section.ID())

	collections := slices.Collect(func(yield func(s string) bool) {
		for _, c := range section.Document().Collections() {
			if !yield(string(c.ID())) {
				return
			}
		}
	})

	content, err := section.Content()
	if err != nil {
		return errors.WithStack(err)
	}

	if len(content) == 0 {
		slog.DebugContext(ctx, "ignoring empty section", slog.String("sectionID", string(section.ID())))
		return nil
	}

	data := map[string]any{
		"_type":       "resource",
		"content":     string(content),
		"source":      source.String(),
		"collections": collections,
	}

	slog.DebugContext(ctx, "indexing section", slog.String("sectionID", string(section.ID())), slog.Int("sectionSize", len(content)))

	if err := i.index.Index(sectionID.String(), data); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Search implements port.Index.
func (i *Index) Search(ctx context.Context, query string, opts port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	queries := []bleveQuery.Query{}

	matchQuery := bleve.NewMatchQuery(query)
	queries = append(queries, matchQuery)

	if len(opts.Collections) > 0 {
		collectionQueries := make([]bleveQuery.Query, 0)
		for _, c := range opts.Collections {
			termQuery := bleve.NewTermQuery(string(c))
			termQuery.SetField("collections")
			collectionQueries = append(collectionQueries, termQuery)
		}
		queries = append(queries, bleve.NewDisjunctionQuery(collectionQueries...))
	}

	req := bleve.NewSearchRequest(bleve.NewConjunctionQuery(queries...))

	req.From = 0

	if opts.MaxResults > 0 {
		req.Size = opts.MaxResults
	}

	result, err := i.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mappedScores := map[string]float64{}
	mappedSections := map[string][]model.SectionID{}

	for _, r := range result.Hits {
		source, err := url.Parse(r.ID)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		sectionID := model.SectionID(source.Fragment)

		source.Fragment = ""

		sectionIDs, exists := mappedSections[source.String()]
		if !exists {
			sectionIDs = make([]model.SectionID, 0)
		}

		sectionIDs = append(sectionIDs, sectionID)

		mappedSections[source.String()] = sectionIDs
		mappedScores[source.String()] += r.Score
	}

	searchResults := make([]*port.IndexSearchResult, 0)

	for rawSource, sectionIDs := range mappedSections {
		source, err := url.Parse(rawSource)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		searchResults = append(searchResults, &port.IndexSearchResult{
			Source:   source,
			Sections: sectionIDs,
		})
	}

	slices.SortFunc(searchResults, func(r1 *port.IndexSearchResult, r2 *port.IndexSearchResult) int {
		score1 := mappedScores[r1.Source.String()]
		score2 := mappedScores[r2.Source.String()]
		if score1 > score2 {
			return -1
		}
		if score1 < score2 {
			return 1
		}
		return 0
	})

	return searchResults, nil
}

func NewIndex(index bleve.Index) *Index {
	return &Index{
		index: index,
	}
}

var _ port.Index = &Index{}
