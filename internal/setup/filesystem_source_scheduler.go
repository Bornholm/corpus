package setup

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/corpus/internal/config"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

// startFilesystemSourceScheduler lances a background goroutine that triggers
// automatic syncs for filesystem sources that have a SyncInterval configured.
func startFilesystemSourceScheduler(ctx context.Context, conf *config.Config, taskRunner port.TaskRunner, sourceStore port.FilesystemSourceStore) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := scheduleOverdueSyncs(ctx, taskRunner, sourceStore); err != nil {
					slog.ErrorContext(ctx, "could not schedule overdue filesystem source syncs", slog.Any("error", errors.WithStack(err)))
				}
			}
		}
	}()
}

func scheduleOverdueSyncs(ctx context.Context, taskRunner port.TaskRunner, sourceStore port.FilesystemSourceStore) error {
	sources, _, err := sourceStore.QueryFilesystemSources(ctx, 0, 1000)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, src := range sources {
		interval := src.SyncInterval()
		if interval == nil {
			continue
		}

		// Skip if the last sync is recent enough
		if src.LastSyncAt() != nil && time.Since(*src.LastSyncAt()) < *interval {
			continue
		}

		// Skip if last sync task is still running or pending
		if src.LastSyncTaskID() != nil {
			state, err := taskRunner.GetTaskState(ctx, *src.LastSyncTaskID())
			if err == nil && (state.Status == port.TaskStatusPending || state.Status == port.TaskStatusRunning) {
				continue
			}
		}

		ctx := slogx.WithAttrs(ctx,
			slog.String("sourceID", string(src.ID())),
			slog.String("sourceLabel", src.Label()),
		)

		syncTask := documentTask.NewSyncFilesystemSourceTask(nil, src.ID())
		if err := taskRunner.ScheduleTask(ctx, syncTask); err != nil {
			slog.ErrorContext(ctx, "could not schedule auto-sync task", slog.Any("error", errors.WithStack(err)))
			continue
		}

		slog.InfoContext(ctx, "scheduled automatic filesystem source sync", slog.String("taskID", string(syncTask.ID())))
	}

	return nil
}
