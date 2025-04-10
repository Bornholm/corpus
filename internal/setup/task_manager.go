package setup

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/metrics"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var TaskManager = NewRegistry[port.TaskManager]()

var getTaskManager = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (port.TaskManager, error) {
	taskManager, err := TaskManager.From(conf.TaskManager.URI)
	if err != nil {
		return nil, errors.Wrapf(err, "could not retrieve file converter for uri '%s'", conf.TaskManager.URI)
	}

	go func() {
		taskManagerCtx := context.Background()
		backoff := time.Second
		for {
			start := time.Now()
			if err := taskManager.Run(taskManagerCtx); err != nil {
				slog.ErrorContext(taskManagerCtx, "error while running task manager", slog.Any("error", errors.WithStack(err)))
			}
			time.Sleep(backoff)
			if time.Now().Sub(start) > backoff/2 {
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
			tasks, err := taskManager.List(ctx)
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
				metrics.TotalTasks.With(prometheus.Labels{
					metrics.LabelStatus: string(status),
				}).Set(total)
			}

			<-ticker.C
		}
	}()

	return taskManager, nil
})
