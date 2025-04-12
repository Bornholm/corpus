package pipeline

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

// Delete duplicated content originating from the same
// branch in the document
type DuplicateContentResultsTransformer struct {
	store port.Store
}

// TransformResults implements ResultsTransformer.
func (t *DuplicateContentResultsTransformer) TransformResults(ctx context.Context, query string, results []*port.IndexSearchResult) ([]*port.IndexSearchResult, error) {
	merged := make([]*port.IndexSearchResult, 0, len(results))

	for _, r := range results {
		updated := &port.IndexSearchResult{
			Source:   r.Source,
			Sections: make([]model.SectionID, 0),
		}

		sections := make([]model.Section, 0, len(r.Sections))

		for _, sectionID := range r.Sections {
			s, err := t.store.GetSectionByID(ctx, sectionID)
			if err != nil {
				if errors.Is(err, port.ErrNotFound) {
					continue
				}

				return nil, errors.WithStack(err)
			}

			sections = append(sections, s)
		}

		childrenOnly := make([]model.Section, 0)

		for _, s := range sections {
			// Retrieve the number of its own children present
			// if they are at least one, use children instead of this parent
			if children := getChildren(sections, s); len(children) > 0 {
				continue
			}

			childrenOnly = append(childrenOnly, s)
		}

		for _, s := range childrenOnly {
			if hasAncestor(childrenOnly, s) {
				continue
			}

			updated.Sections = append(updated.Sections, s.ID())
		}

		merged = append(merged, updated)
	}

	return merged, nil
}

func NewDuplicateContentResultsTransformer(store port.Store) *DuplicateContentResultsTransformer {
	return &DuplicateContentResultsTransformer{
		store: store,
	}
}

var _ ResultsTransformer = &DuplicateContentResultsTransformer{}

func getChildren(sections []model.Section, parent model.Section) []model.Section {
	children := make([]model.Section, 0)
	for _, s := range sections {
		if parent.ID() == s.ID() {
			continue
		}

		if isParentOf(parent, s) {
			children = append(children, s)
		}
	}
	return children
}

func isParentOf(section model.Section, child model.Section) bool {
	branch := child.Branch()
	length := len(branch)
	if length < 2 {
		return false
	}

	if branch[length-2] != section.ID() {
		return false
	}

	return true
}

func hasAncestor(sections []model.Section, section model.Section) bool {
	for _, other := range sections {
		if other.ID() == section.ID() {
			continue
		}

		if isAncestor(other, section) {
			return true
		}
	}

	return false
}

func isAncestor(parent model.Section, child model.Section) bool {
	for i, p := range parent.Branch() {
		branch := child.Branch()
		if i >= len(branch) || p != branch[i] {
			return false
		}
	}

	return true
}
