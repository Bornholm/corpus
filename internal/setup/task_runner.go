package setup

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service/backup"
	"github.com/bornholm/corpus/internal/metrics"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var TaskRunner = NewRegistry[port.TaskRunner]()

var getTaskRunner = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.TaskRunner, error) {
	taskRunner, err := TaskRunner.From(conf.TaskRunner.URI)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve task runner for uri '%s'", conf.TaskRunner.URI)
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

	return nil
}
