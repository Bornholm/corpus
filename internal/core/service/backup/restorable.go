package backup

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
)

type Restorable interface {
	RestoreDocuments(ctx context.Context, documents []model.Document) error
}
