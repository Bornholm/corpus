package setup

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
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

	return taskManager, nil
})
