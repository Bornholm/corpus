package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bornholm/corpus/pkg/model"
)

type FilesystemSourceUpdates struct {
	Label         *string
	BackendType   *string
	BackendConfig *json.RawMessage
	CollectionIDs []model.CollectionID
	Options       *model.FilesystemSourceOptions
	// Double pointer: nil = don't update, non-nil pointer to nil = clear interval
	SyncInterval **time.Duration
}

type FilesystemSourceStore interface {
	CreateFilesystemSource(ctx context.Context, label string, backendType string, backendConfig json.RawMessage, collectionIDs []model.CollectionID, opts model.FilesystemSourceOptions, syncInterval *time.Duration) (model.FilesystemSource, error)
	GetFilesystemSourceByID(ctx context.Context, id model.FilesystemSourceID) (model.FilesystemSource, error)
	QueryFilesystemSources(ctx context.Context, page, limit int) ([]model.FilesystemSource, int64, error)
	UpdateFilesystemSource(ctx context.Context, id model.FilesystemSourceID, updates FilesystemSourceUpdates) (model.FilesystemSource, error)
	DeleteFilesystemSource(ctx context.Context, id model.FilesystemSourceID) error
	UpdateFilesystemSourceSyncState(ctx context.Context, id model.FilesystemSourceID, lastSyncAt time.Time, taskID model.TaskID) error
}
