package gorm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type FilesystemSource struct {
	ID             string     `gorm:"primaryKey;autoIncrement:false"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Label          string
	BackendType    string  `gorm:"column:backend_type"`
	BackendConfig  []byte  `gorm:"column:backend_config"`
	CollectionIDs  string  `gorm:"column:collection_ids"`
	OptionsJSON    []byte  `gorm:"column:options"`
	LastSyncAt     *time.Time
	LastSyncTaskID *string `gorm:"column:last_sync_task_id"`
	SyncIntervalNs *int64  `gorm:"column:sync_interval_ns"`
}

func (r *FilesystemSource) toModel() (model.FilesystemSource, error) {
	var collIDs []model.CollectionID
	if r.CollectionIDs != "" {
		if err := json.Unmarshal([]byte(r.CollectionIDs), &collIDs); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	opts := model.DefaultFilesystemSourceOptions()
	if len(r.OptionsJSON) > 0 {
		if err := json.Unmarshal(r.OptionsJSON, &opts); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	var lastSyncTaskID *model.TaskID
	if r.LastSyncTaskID != nil {
		taskID := model.TaskID(*r.LastSyncTaskID)
		lastSyncTaskID = &taskID
	}

	var syncInterval *time.Duration
	if r.SyncIntervalNs != nil {
		d := time.Duration(*r.SyncIntervalNs)
		syncInterval = &d
	}

	return model.NewFilesystemSource(
		model.FilesystemSourceID(r.ID),
		r.Label,
		r.BackendType,
		json.RawMessage(r.BackendConfig),
		collIDs,
		opts,
		r.LastSyncAt,
		lastSyncTaskID,
		syncInterval,
	), nil
}

// CreateFilesystemSource implements port.FilesystemSourceStore.
func (s *Store) CreateFilesystemSource(ctx context.Context, label string, backendType string, backendConfig json.RawMessage, collectionIDs []model.CollectionID, opts model.FilesystemSourceOptions, syncInterval *time.Duration) (model.FilesystemSource, error) {
	collIDsJSON, err := json.Marshal(collectionIDs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var syncIntervalNs *int64
	if syncInterval != nil {
		ns := int64(*syncInterval)
		syncIntervalNs = &ns
	}

	record := &FilesystemSource{
		ID:             string(model.NewFilesystemSourceID()),
		Label:          label,
		BackendType:    backendType,
		BackendConfig:  []byte(backendConfig),
		CollectionIDs:  string(collIDsJSON),
		OptionsJSON:    optsJSON,
		SyncIntervalNs: syncIntervalNs,
	}

	err = s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
		return db.Create(record).Error
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return record.toModel()
}

// GetFilesystemSourceByID implements port.FilesystemSourceStore.
func (s *Store) GetFilesystemSourceByID(ctx context.Context, id model.FilesystemSourceID) (model.FilesystemSource, error) {
	var record FilesystemSource

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		return db.First(&record, "id = ?", string(id)).Error
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.WithStack(port.ErrNotFound)
		}
		return nil, errors.WithStack(err)
	}

	return record.toModel()
}

// QueryFilesystemSources implements port.FilesystemSourceStore.
func (s *Store) QueryFilesystemSources(ctx context.Context, page, limit int) ([]model.FilesystemSource, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	var records []FilesystemSource
	var total int64

	err := s.withRetry(ctx, false, func(ctx context.Context, db *gorm.DB) error {
		if err := db.Model(&FilesystemSource{}).Count(&total).Error; err != nil {
			return errors.WithStack(err)
		}
		return db.Order("created_at DESC").Offset(page * limit).Limit(limit).Find(&records).Error
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, 0, errors.WithStack(err)
	}

	sources := make([]model.FilesystemSource, 0, len(records))
	for _, r := range records {
		src, err := r.toModel()
		if err != nil {
			return nil, 0, errors.WithStack(err)
		}
		sources = append(sources, src)
	}

	return sources, total, nil
}

// UpdateFilesystemSource implements port.FilesystemSourceStore.
func (s *Store) UpdateFilesystemSource(ctx context.Context, id model.FilesystemSourceID, updates port.FilesystemSourceUpdates) (model.FilesystemSource, error) {
	var record FilesystemSource

	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
		if err := db.First(&record, "id = ?", string(id)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.WithStack(port.ErrNotFound)
			}
			return errors.WithStack(err)
		}

		fields := map[string]any{}

		if updates.Label != nil {
			record.Label = *updates.Label
			fields["label"] = record.Label
		}
		if updates.BackendType != nil {
			record.BackendType = *updates.BackendType
			fields["backend_type"] = record.BackendType
		}
		if updates.BackendConfig != nil {
			record.BackendConfig = []byte(*updates.BackendConfig)
			fields["backend_config"] = record.BackendConfig
		}
		if updates.CollectionIDs != nil {
			collIDsJSON, err := json.Marshal(updates.CollectionIDs)
			if err != nil {
				return errors.WithStack(err)
			}
			record.CollectionIDs = string(collIDsJSON)
			fields["collection_ids"] = record.CollectionIDs
		}
		if updates.Options != nil {
			optsJSON, err := json.Marshal(*updates.Options)
			if err != nil {
				return errors.WithStack(err)
			}
			record.OptionsJSON = optsJSON
			fields["options"] = record.OptionsJSON
		}
		if updates.SyncInterval != nil {
			if *updates.SyncInterval == nil {
				record.SyncIntervalNs = nil
			} else {
				ns := int64(**updates.SyncInterval)
				record.SyncIntervalNs = &ns
			}
			fields["sync_interval_ns"] = record.SyncIntervalNs
		}

		if len(fields) == 0 {
			return nil
		}

		return db.Model(&record).Updates(fields).Error
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return record.toModel()
}

// DeleteFilesystemSource implements port.FilesystemSourceStore.
func (s *Store) DeleteFilesystemSource(ctx context.Context, id model.FilesystemSourceID) error {
	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
		result := db.Delete(&FilesystemSource{}, "id = ?", string(id))
		if result.Error != nil {
			return errors.WithStack(result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.WithStack(port.ErrNotFound)
		}
		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	return errors.WithStack(err)
}

// UpdateFilesystemSourceSyncState implements port.FilesystemSourceStore.
func (s *Store) UpdateFilesystemSourceSyncState(ctx context.Context, id model.FilesystemSourceID, lastSyncAt time.Time, taskID model.TaskID) error {
	taskIDStr := string(taskID)
	err := s.withRetry(ctx, true, func(ctx context.Context, db *gorm.DB) error {
		return db.Model(&FilesystemSource{}).Where("id = ?", string(id)).Updates(map[string]any{
			"last_sync_at":      lastSyncAt,
			"last_sync_task_id": taskIDStr,
		}).Error
	}, sqlite3.LOCKED, sqlite3.BUSY)
	return errors.WithStack(err)
}

var _ port.FilesystemSourceStore = &Store{}
