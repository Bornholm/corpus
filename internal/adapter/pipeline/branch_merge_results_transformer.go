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

		sections := make([]model.Section, 0, len(r.Sections))

		for _, sectionID := range r.Sections {
			s, err := t.store.GetSectionBySourceAndID(ctx, r.Source, sectionID)
			if err != nil {
				if errors.Is(err, port.ErrNotFound) {
					continue
				}

				return nil, errors.WithStack(err)
			}

			sections = append(sections, s)
		}

		for _, s := range sections {
			ancestor := findAncestor(sections, s)
			if ancestor == nil {
				// If the result contains a section and its children (more than one)
				// include only children
				if children := getChildren(sections, s); len(children) > 1 {
					continue
				}
			}

			updated.Sections = append(updated.Sections, s.ID())
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

func hasSiblings(sections []model.Section, ancestor model.SectionID, section model.Section) bool {
	for _, other := range sections {
		if other.ID() == section.ID() {
			continue
		}
	}

	return false
}

func getChildren(sections []model.Section, parent model.Section) []model.Section {
	children := make([]model.Section, 0)
	for _, s := range sections {
		if isAncestor(parent.Branch(), s.Branch()) {
			children = append(children, s)
		}
	}
	return children
}

func findAncestor(sections []model.Section, section model.Section) model.Section {
	var ancestor model.Section
	for _, other := range sections {
		if other.ID() == section.ID() {
			continue
		}

		if isAncestor(other.Branch(), section.Branch()) && (ancestor == nil || len(other.Branch()) < len(ancestor.Branch())) {
			ancestor = other
		}
	}

	return ancestor
}

func isAncestor(parentBranch []model.SectionID, childBranch []model.SectionID) bool {
	for i, p := range parentBranch {
		if i >= len(childBranch) || p != childBranch[i] {
			return false
		}
	}

	return true
}
