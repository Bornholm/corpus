package backup

import (
	"context"

	"github.com/bornholm/corpus/pkg/model"
)

type Restorable interface {
	RestoreDocuments(ctx context.Context, documents []model.Document) error
}
