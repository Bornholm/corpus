package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/bornholm/corpus/internal/backup"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/xid"
)

type BackupManager struct {
	index      port.Index
	store      port.Store
	taskRunner port.TaskRunner
}

func (m *BackupManager) Backup(ctx context.Context) (io.ReadCloser, error) {
	snapshotable := m.getCompositeSnapshotable()

	reader, err := snapshotable.GenerateSnapshot(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return reader, nil
}

func (m *BackupManager) RestoreBackup(ctx context.Context, r io.Reader) (port.TaskID, error) {
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

	taskID := port.NewTaskID()

	restoreBackup := &restoreBackupTask{
		id:   taskID,
		path: path,
	}

	taskCtx := log.WithAttrs(context.Background(), slog.String("path", path))

	if err := m.taskRunner.Schedule(taskCtx, restoreBackup); err != nil {
		return "", errors.WithStack(err)
	}

	return taskID, nil

}

func (m *BackupManager) handleRestoreBackupTaskfunc(ctx context.Context, task port.Task, events chan port.TaskEvent) error {
	restoreBackupTask, ok := task.(*restoreBackupTask)
	if !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	defer func() {
		events <- port.NewTaskEvent(port.WithTaskProgress(1))
	}()

	defer func() {
		if err := os.Remove(restoreBackupTask.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.ErrorContext(ctx, "could not remove file", slog.Any("error", errors.WithStack(err)))
		}
	}()

	backupFile, err := os.Open(restoreBackupTask.path)
	if err != nil {
		return errors.Wrap(err, "could not open backup file")
	}

	defer backupFile.Close()

	snapshotable := m.getCompositeSnapshotable()

	slog.InfoContext(ctx, "restoring snapshot")
	events <- port.NewTaskEvent(port.WithTaskMessage("restoring snapshots"))

	if err := snapshotable.RestoreSnapshot(ctx, backupFile); err != nil {
		return errors.WithStack(err)
	}

	restorable := m.getRestorables()

	events <- port.NewTaskEvent(port.WithTaskMessage("restoring documents"), port.WithTaskProgress(0.5))

	page := 0
	limit := 100
	for {
		slog.DebugContext(ctx, "querying documents to restore", slog.Int("page", page), slog.Int("limit", limit))

		documents, total, err := m.store.QueryDocuments(ctx, port.QueryDocumentsOptions{Page: &page, Limit: &limit})
		if err != nil {
			return errors.WithStack(err)
		}

		progress := float64(page*limit) / float64(total)
		events <- port.NewTaskEvent(port.WithTaskProgress(float32((0.5 + progress/2))))

		if len(documents) == 0 {
			return nil
		}

		slog.InfoContext(ctx, "restoring documents", slog.Int64("totalDocuments", total), slog.Int("progressPercent", int(progress*100)))

		for _, r := range restorable {
			if err := r.RestoreDocuments(ctx, documents); err != nil {
				return errors.WithStack(err)
			}
		}

		page++
	}
}

func (m *BackupManager) getCompositeSnapshotable() *backup.Composite {
	snapshotables := make([]backup.IdentifiedSnapshotable, 0)

	if snapshotableIndex, ok := m.index.(backup.Snapshotable); ok {
		snapshotables = append(snapshotables, backup.WithSnapshotID("index-v1", snapshotableIndex))
	}

	if snapshotableStore, ok := m.store.(backup.Snapshotable); ok {
		snapshotables = append(snapshotables, backup.WithSnapshotID("store-v1", snapshotableStore))
	}

	return backup.ComposeSnapshots(snapshotBoundary, snapshotables...)
}

func (m *BackupManager) getRestorables() []Restorable {
	restorables := make([]Restorable, 0)

	if restorableIndex, ok := m.index.(Restorable); ok {
		restorables = append(restorables, restorableIndex)
	}

	return restorables
}

func NewBackupManager(index port.Index, store port.Store, taskRunner port.TaskRunner) *BackupManager {
	backupManager := &BackupManager{index, store, taskRunner}

	taskRunner.Register(restoreBackupTaskType, port.TaskHandlerFunc(backupManager.handleRestoreBackupTaskfunc))

	return backupManager
}

const restoreBackupTaskType port.TaskType = "restoreBackup"

type restoreBackupTask struct {
	id   port.TaskID
	path string
}

// ID implements port.Task.
func (i *restoreBackupTask) ID() port.TaskID {
	return i.id
}

// Type implements port.Task.
func (i *restoreBackupTask) Type() port.TaskType {
	return restoreBackupTaskType
}

var _ port.Task = &restoreBackupTask{}
