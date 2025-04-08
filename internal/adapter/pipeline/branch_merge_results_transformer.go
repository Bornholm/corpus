package pipeline

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

// Delete duplicated content originating from the same
// branch in the document
type BranchMergeResultsTransformer struct {
	store port.Store
}

// TransformResults implements ResultsTransformer.
func (t *BranchMergeResultsTransformer) TransformResults(ctx context.Context, query string, results []*port.IndexSearchResult) ([]*port.IndexSearchResult, error) {
	merged := make([]*port.IndexSearchResult, 0, len(results))

	for _, r := range results {
		updated := &port.IndexSearchResult{
			Source:   r.Source,
			Sections: make([]model.SectionID, 0),
		}

		for _, sectionID := range r.Sections {
			section, err := t.store.GetSectionBySourceAndID(ctx, r.Source, sectionID)
			if err != nil {
				if errors.Is(err, port.ErrNotFound) {
					continue
				}

				return nil, errors.WithStack(err)
			}

			if len(section.Branch()) > 1 {
				updated.Sections = append(updated.Sections, sectionID)
			}
		}

		merged = append(merged, updated)
	}

	return merged, nil
}

func NewBranchMergeResultsTransformer(store port.Store) *BranchMergeResultsTransformer {
	return &BranchMergeResultsTransformer{
		store: store,
	}
}

var _ ResultsTransformer = &BranchMergeResultsTransformer{}
