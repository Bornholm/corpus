package setup

import (
	"context"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/service/backup"
	"github.com/bornholm/corpus/internal/metrics"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	gormAdapter "github.com/bornholm/corpus/pkg/adapter/gorm"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var TaskRunner = NewRegistry[port.TaskRunner]()

var getTaskRunner = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.TaskRunner, error) {
	var taskRunner port.TaskRunner

	u, err := url.Parse(conf.TaskRunner.URI)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse task runner uri '%s'", conf.TaskRunner.URI)
	}

	if u.Scheme == "sqlite" {
		// Task runner persistant GORM — réutilise la même DB que le store.
		db, err := getGormDatabaseFromConfig(ctx, conf)
		if err != nil {
			return nil, errors.Wrap(err, "could not get gorm database for persistent task runner")
		}

		parallelism := 5
		if v := u.Query().Get("parallelism"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				parallelism = n
			}
		}

		cleanupDelay := 60 * time.Minute
		if v := u.Query().Get("cleanupDelay"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				cleanupDelay = d
			}
		}

		cleanupInterval := 10 * time.Minute
		if v := u.Query().Get("cleanupInterval"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				cleanupInterval = d
			}
		}

		taskRunner = gormAdapter.NewGormTaskRunner(db, parallelism, cleanupDelay, cleanupInterval)
	} else {
		taskRunner, err = TaskRunner.From(conf.TaskRunner.URI)
		if err != nil {
			return nil, errors.Wrapf(err, "could not retrieve task runner for uri '%s'", conf.TaskRunner.URI)
		}
	}

	go func() {
		taskRunnerCtx := context.Background()
		backoff := time.Second
		for {
			start := time.Now()
			if err := taskRunner.Run(taskRunnerCtx); err != nil {
				slog.ErrorContext(taskRunnerCtx, "error while running task runner", slog.Any("error", errors.WithStack(err)))
			}
			time.Sleep(backoff)
			if time.Since(start) > backoff/2 {
				backoff = time.Second
			} else {
				backoff *= 2
			}
		}
	}()

	// Collect tasks metrics
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		ctx := context.Background()
		for {
			tasks, err := taskRunner.ListTasks(ctx)
			if err != nil {
				slog.ErrorContext(ctx, "could not list tasks", slog.Any("error", errors.WithStack(err)))
				continue
			}

			stats := map[port.TaskStatus]float64{
				port.TaskStatusPending:   0,
				port.TaskStatusRunning:   0,
				port.TaskStatusFailed:    0,
				port.TaskStatusSucceeded: 0,
			}
			for _, t := range tasks {
				stats[t.Status] += 1
			}

			for status, total := range stats {
				metrics.Tasks.With(prometheus.Labels{
					metrics.LabelStatus: string(status),
				}).Set(total)
			}

			<-ticker.C
		}
	}()

	return taskRunner, nil
})

func setupTaskHandlers(ctx context.Context, conf *config.Config, taskRunner port.TaskRunner) error {
	// Enregistrement des factories de désérialisation pour le task runner persistant.
	if persistentRunner, ok := taskRunner.(port.PersistentTaskRunner); ok {
		persistentRunner.RegisterFactory(documentTask.TaskTypeIndexFile, documentTask.RestoreIndexFileTask)
		persistentRunner.RegisterFactory(documentTask.TaskTypeCleanup, documentTask.RestoreCleanupTask)
		persistentRunner.RegisterFactory(documentTask.TaskTypeReindexCollection, documentTask.RestoreReindexCollectionTask)
		persistentRunner.RegisterFactory(documentTask.TaskTypeReindexBleve, documentTask.RestoreReindexBleveTask)
		persistentRunner.RegisterFactory(backup.TaskTypeRestoreBackup, backup.RestoreRestoreBackupTask)
	}

	indexFileHandler, err := getIndexFileTaskHandler(ctx, conf)
	if err != nil {
		return errors.Wrap(err, "could not create index file task handler from config")
	}

	taskRunner.RegisterTask(documentTask.TaskTypeIndexFile, indexFileHandler)

	restoreBackupHandler, err := getRestoreBackupTaskHandler(ctx, conf)
	if err != nil {
		return errors.Wrap(err, "could not create index file task handler from config")
	}

	taskRunner.RegisterTask(backup.TaskTypeRestoreBackup, restoreBackupHandler)

	cleanupHandler, err := getCleanupTaskHandler(ctx, conf)
	if err != nil {
		return errors.Wrap(err, "could not cleanup task handler from config")
	}

	taskRunner.RegisterTask(documentTask.TaskTypeCleanup, cleanupHandler)

	reindexCollectionHandler, err := getReindexCollectionTaskHandler(ctx, conf)
	if err != nil {
		return errors.Wrap(err, "could not reindex collection task handler from config")
	}

	taskRunner.RegisterTask(documentTask.TaskTypeReindexCollection, reindexCollectionHandler)

	reindexBleveHandler, err := getReindexBleveTaskHandler(ctx, conf)
	if err != nil {
		return errors.Wrap(err, "could not reindex bleve task handler from config")
	}

	taskRunner.RegisterTask(documentTask.TaskTypeReindexBleve, reindexBleveHandler)

	// Schedule bleve reindex if a mapping change was detected during startup.
	// This is done here, after all handlers are registered, to avoid a race where
	// the task goroutine fires before TaskTypeReindexBleve has a handler.
	if bleveReindexNeeded {
		bleveReindexNeeded = false
		reindexTask := documentTask.NewReindexBleveTask(nil)
		if err := taskRunner.ScheduleTask(ctx, reindexTask); err != nil {
			slog.ErrorContext(ctx, "could not schedule bleve reindex task", slog.Any("error", errors.WithStack(err)))
		} else {
			slog.InfoContext(ctx, "scheduled bleve index reindex task", slog.String("task_id", string(reindexTask.ID())))
		}
	}

	return nil
}
