package bleve

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

// All implements port.Index.
func (i *Index) All(ctx context.Context, yield func(model.SectionID) bool) error {
	advanced, err := i.index.Advanced()
	if err != nil {
		return errors.WithStack(err)
	}

	reader, err := advanced.Reader()
	if err != nil {
		return errors.WithStack(err)
	}

	ids, err := reader.DocIDReaderAll()
	if err != nil {
		return errors.WithStack(err)
	}

	for {
		internalID, err := ids.Next()
		if err != nil {
			return errors.WithStack(err)
		}

		if internalID == nil {
			return nil
		}

		id, err := reader.ExternalID(internalID)
		if err != nil {
			return errors.WithStack(err)
		}

		if !yield(model.SectionID(id)) {
			return nil
		}
	}
}
