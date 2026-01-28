package pipeline

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/service/backup"
	"github.com/pkg/errors"
)

// RestoreDocuments implements service.Restorable.
func (i *Index) RestoreDocuments(ctx context.Context, documents []model.Document) error {
	restorables := i.getRestorables()
	for _, r := range restorables {
		if err := r.RestoreDocuments(ctx, documents); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (i *Index) getRestorables() []backup.Restorable {
	restorables := make([]backup.Restorable, 0)

	for index := range i.indexes {
		if r, ok := index.Index().(backup.Restorable); ok {
			restorables = append(restorables, r)
		}
	}

	return restorables
}

var _ backup.Restorable = &Index{}
