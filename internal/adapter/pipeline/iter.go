package pipeline

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

// All implements port.Index.
func (i *Index) All(ctx context.Context, yield func(model.SectionID) bool) error {
	for index := range i.indexes {
		if err := index.Index().All(ctx, yield); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}
