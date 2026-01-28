package backup

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/bornholm/corpus/internal/backup"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/xid"
)

const snapshotBoundary = "corpus-snapshot-v1"

type Manager struct {
	index      port.Index
	store      port.DocumentStore
	taskRunner port.TaskRunner
}

func (m *Manager) Backup(ctx context.Context) (io.ReadCloser, error) {
	snapshotable := m.GetCompositeSnapshotable()

	reader, err := snapshotable.GenerateSnapshot(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return reader, nil
}

func (m *Manager) RestoreBackup(ctx context.Context, owner model.User, r io.Reader) (model.TaskID, error) {
	tempDir, err := util.TempDir()
	if err != nil {
		return "", errors.WithStack(err)
	}

	path := filepath.Join(tempDir, fmt.Sprintf("backup_%s.bin.gz", xid.New().String()))

	file, err := os.Create(path)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if _, err := io.Copy(file, r); err != nil {
		return "", errors.WithStack(err)
	}

	taskID := model.NewTaskID()

	restoreBackup := NewRestoreBackupTask(owner, path)

	taskCtx := log.WithAttrs(context.Background(), slog.String("path", path))

	if err := m.taskRunner.Schedule(taskCtx, restoreBackup); err != nil {
		return "", errors.WithStack(err)
	}

	return taskID, nil

}

func (m *Manager) GetCompositeSnapshotable() *backup.Composite {
	snapshotables := make([]backup.IdentifiedSnapshotable, 0)

	if snapshotableIndex, ok := m.index.(backup.Snapshotable); ok {
		snapshotables = append(snapshotables, backup.WithSnapshotID("index-v1", snapshotableIndex))
	}

	if snapshotableStore, ok := m.store.(backup.Snapshotable); ok {
		snapshotables = append(snapshotables, backup.WithSnapshotID("store-v1", snapshotableStore))
	}

	return backup.ComposeSnapshots(snapshotBoundary, snapshotables...)
}

func (m *Manager) GetRestorables() []Restorable {
	restorables := make([]Restorable, 0)

	if restorableIndex, ok := m.index.(Restorable); ok {
		restorables = append(restorables, restorableIndex)
	}

	return restorables
}

func NewManager(index port.Index, store port.DocumentStore, taskRunner port.TaskRunner) *Manager {
	backupManager := &Manager{index, store, taskRunner}

	return backupManager
}
