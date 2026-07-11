package document

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	fsbackend "github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/bornholm/corpus/internal/filesystem/reconciler"
	"github.com/bornholm/corpus/internal/util"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
	"github.com/rs/xid"
	"github.com/spf13/afero"

	// Filesystem backends
	_ "github.com/bornholm/corpus/internal/filesystem/backend/ftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/git"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/local"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/minio"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/sftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/smb"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/webdav"
)

type SyncFilesystemSourceHandler struct {
	sourceStore   port.FilesystemSourceStore
	documentStore port.DocumentStore
	taskRunner    port.TaskRunner
}

func NewSyncFilesystemSourceHandler(sourceStore port.FilesystemSourceStore, documentStore port.DocumentStore, taskRunner port.TaskRunner) *SyncFilesystemSourceHandler {
	return &SyncFilesystemSourceHandler{
		sourceStore:   sourceStore,
		documentStore: documentStore,
		taskRunner:    taskRunner,
	}
}

// Handle implements port.TaskHandler.
func (h *SyncFilesystemSourceHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	t, ok := task.(*SyncFilesystemSourceTask)
	if !ok {
		return errors.Errorf("unexpected task type '%T'", task)
	}

	ctx = slogx.WithAttrs(ctx, slog.String("sourceID", string(t.sourceID)))

	source, err := h.sourceStore.GetFilesystemSourceByID(ctx, t.sourceID)
	if err != nil {
		return errors.Wrapf(err, "could not load filesystem source '%s'", t.sourceID)
	}

	ctx = slogx.WithAttrs(ctx, slog.String("sourceLabel", source.Label()), slog.String("backendType", source.BackendType()))
	slog.InfoContext(ctx, "starting filesystem source sync")

	backend, err := fsbackend.NewFromConfig(source.BackendType(), source.BackendConfig())
	if err != nil {
		return errors.Wrapf(err, "could not create backend of type '%s'", source.BackendType())
	}

	opts := source.Options()
	reconcilerOpts := reconciler.Options{
		Directory:      opts.Directory,
		SourceTemplate: opts.SourceTemplate,
		ETagStrategy:   opts.ETagStrategy,
	}

	return backend.Mount(ctx, func(ctx context.Context, afs afero.Fs) error {
		toIndex, toDelete, err := reconciler.Reconcile(ctx, afs, h.documentStore, reconcilerOpts)
		if err != nil {
			return errors.WithStack(err)
		}

		total := len(toIndex) + len(toDelete)
		done := 0

		sendProgress := func() {
			if total > 0 {
				p := float32(done) / float32(total)
				events <- port.NewTaskEvent(port.WithTaskProgress(p))
			}
		}

		concurrency := opts.Concurrency
		if concurrency <= 0 {
			concurrency = 8
		}

		jobCh := make(chan reconciler.IndexJob, concurrency)
		var wg sync.WaitGroup

		for w := 0; w < concurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobCh {
					if err := h.stageAndScheduleIndexFile(ctx, task.Owner(), afs, job, source.CollectionIDs()); err != nil {
						slog.ErrorContext(ctx, "could not stage file for indexing",
							slog.Any("error", err),
							slog.String("path", job.Path))
					}
					done++
					sendProgress()
				}
			}()
		}

		for _, job := range toIndex {
			jobCh <- job
		}
		close(jobCh)
		wg.Wait()

		if opts.DeleteOrphans && len(toDelete) > 0 {
			if err := h.documentStore.DeleteDocumentByID(ctx, toDelete...); err != nil {
				slog.ErrorContext(ctx, "could not delete orphan documents", slog.Any("error", errors.WithStack(err)))
			}
			done += len(toDelete)
			sendProgress()
		}

		if err := h.sourceStore.UpdateFilesystemSourceSyncState(ctx, source.ID(), time.Now(), task.ID()); err != nil {
			slog.ErrorContext(ctx, "could not update sync state", slog.Any("error", errors.WithStack(err)))
		}

		slog.InfoContext(ctx, "filesystem source sync complete",
			slog.Int("indexed", len(toIndex)),
			slog.Int("deleted", len(toDelete)),
		)

		return nil
	})
}

// stageAndScheduleIndexFile copies the file from the mounted FS to a temp path
// and schedules an IndexFileTask for it.
func (h *SyncFilesystemSourceHandler) stageAndScheduleIndexFile(ctx context.Context, owner model.User, afs afero.Fs, job reconciler.IndexJob, collectionIDs []model.CollectionID) error {
	f, err := afs.Open(job.Path)
	if err != nil {
		return errors.Wrapf(err, "could not open file '%s'", job.Path)
	}
	defer f.Close()

	tempDir, err := util.TempDir()
	if err != nil {
		return errors.WithStack(err)
	}

	ext := filepath.Ext(job.Filename)
	stagedPath := filepath.Join(tempDir, xid.New().String()+ext)

	dst, err := os.Create(stagedPath)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := io.Copy(dst, f); err != nil {
		dst.Close()
		os.Remove(stagedPath)
		return errors.WithStack(err)
	}
	dst.Close()

	indexTask := NewIndexFileTask(owner, stagedPath, job.Filename, job.ETag, job.Source, collectionIDs)

	if err := h.taskRunner.ScheduleTask(ctx, indexTask); err != nil {
		os.Remove(stagedPath)
		return errors.WithStack(err)
	}

	return nil
}

var _ port.TaskHandler = &SyncFilesystemSourceHandler{}
