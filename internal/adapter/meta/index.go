package meta

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type WeightedIndexes map[port.Index]float64

type Index struct {
	indexes WeightedIndexes
}

type indexSearchResults struct {
	Results []*port.IndexSearchResult
	Index   port.Index
}

// DeleteBySource implements port.Index.
func (i *Index) DeleteBySource(ctx context.Context, source *url.URL) error {
	count := len(i.indexes)
	errs := make(chan error, count)
	defer close(errs)

	var wg sync.WaitGroup

	wg.Add(count)

	for index := range i.indexes {
		go func(index port.Index) {
			defer wg.Done()

			if err := index.DeleteBySource(ctx, source); err != nil {
				errs <- errors.WithStack(err)
				return
			}

			errs <- nil
		}(index)
	}

	wg.Wait()

	err := NewAggregatedError()
	idx := 0

	for e := range errs {
		if e != nil {
			err.Add(e)
		}

		if idx >= count-1 {
			break
		}

		idx++
	}

	if err.Len() > 0 {
		return errors.WithStack(err)
	}

	return nil
}

// Index implements port.Index.
func (i *Index) Index(ctx context.Context, document model.Document) error {
	count := len(i.indexes)
	errs := make(chan error, count)
	defer close(errs)

	var wg sync.WaitGroup

	wg.Add(count)

	for index := range i.indexes {
		go func(index port.Index) {
			defer wg.Done()

			slog.DebugContext(ctx, "indexing document", slog.String("indexType", fmt.Sprintf("%T", index)))

			if err := index.Index(ctx, document); err != nil {
				errs <- errors.WithStack(err)
				return
			}

			errs <- nil
		}(index)
	}

	wg.Wait()

	err := NewAggregatedError()
	idx := 0

	for e := range errs {
		if e != nil {
			err.Add(e)
		}

		if idx >= count-1 {
			break
		}

		idx++
	}

	if err.Len() > 0 {
		return errors.WithStack(err)
	}

	return nil
}

// Search implements port.Index.
func (i *Index) Search(ctx context.Context, query string, opts *port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	count := len(i.indexes)

	type Message struct {
		Results *indexSearchResults
		Err     error
	}

	messages := make(chan *Message, count)
	defer close(messages)

	var wg sync.WaitGroup

	wg.Add(count)

	for index := range i.indexes {
		go func(index port.Index) {
			defer wg.Done()

			results, err := index.Search(ctx, query, &port.IndexSearchOptions{
				MaxResults: opts.MaxResults * 3,
			})
			if err != nil {
				messages <- &Message{
					Err: errors.WithStack(err),
				}
				return
			}

			// log.Printf("Search results for %T: \n\n%s", index, spew.Sdump(results))

			messages <- &Message{
				Results: &indexSearchResults{
					Results: results,
					Index:   index,
				},
			}
		}(index)
	}

	wg.Wait()

	results := make([]*indexSearchResults, 0)
	aggregatedErr := NewAggregatedError()
	idx := 0

	for m := range messages {
		if m.Err != nil {
			aggregatedErr.Add(m.Err)
		}

		if m.Results != nil {
			results = append(results, m.Results)
		}

		if idx >= count-1 {
			break
		}

		idx++
	}

	if aggregatedErr.Len() > 0 {
		return nil, errors.WithStack(aggregatedErr)
	}

	merged, err := i.mergeResults(results, opts.MaxResults)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return merged, nil
}

func (i *Index) mergeResults(indexResults []*indexSearchResults, maxResults int) ([]*port.IndexSearchResult, error) {
	type scoreItem struct {
		Score    float64
		Sections map[model.SectionID]float64
	}

	scores := map[string]scoreItem{}

	for _, r := range indexResults {
		for _, rr := range r.Results {
			source := rr.Source.String()

			var resultScore scoreItem
			if _, exists := scores[source]; !exists {
				scores[source] = scoreItem{
					Score:    0,
					Sections: make(map[model.SectionID]float64),
				}
			}

			resultScore = scores[source]
			resultScore.Score += 1.5 * i.indexes[r.Index]

			for _, s := range rr.Sections {
				if _, exists := resultScore.Sections[s]; !exists {
					resultScore.Sections[s] = 0
				}

				resultScore.Sections[s] += 1 * i.indexes[r.Index]
			}

			scores[source] = resultScore
		}
	}

	sources := []string{}
	for s := range scores {
		sources = append(sources, s)
	}

	slices.SortFunc(sources, func(s1, s2 string) int {
		score1 := scores[s1].Score
		for _, v := range scores[s1].Sections {
			score1 += v
		}

		score2 := scores[s2].Score
		for _, v := range scores[s2].Sections {
			score2 += v
		}

		if score1 < score2 {
			return 1
		}
		if score1 > score2 {
			return -1
		}
		return strings.Compare(s1, s2)
	})

	merged := make([]*port.IndexSearchResult, 0)

	for _, rawSource := range sources {
		source, err := url.Parse(rawSource)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		sectionScores := scores[rawSource].Sections

		sectionIDs := []model.SectionID{}
		for id := range sectionScores {
			sectionIDs = append(sectionIDs, id)
		}

		slices.SortFunc(sectionIDs, func(id1, id2 model.SectionID) int {
			score1 := sectionScores[id1]
			score2 := sectionScores[id2]
			if score1 < score2 {
				return 1
			}
			if score1 > score2 {
				return -1
			}
			return strings.Compare(string(id1), string(id2))
		})

		merged = append(merged, &port.IndexSearchResult{
			Source:   source,
			Sections: sectionIDs,
		})
	}

	if len(merged) > maxResults && maxResults > 1 {
		merged = merged[0 : maxResults-1]
	}

	return merged, nil
}

func NewIndex(indexes WeightedIndexes) *Index {
	return &Index{
		indexes: indexes,
	}
}

var _ port.Index = &Index{}
