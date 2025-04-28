package memory

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bornholm/corpus/internal/adapter/memory/syncx"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/log"
	"github.com/pkg/errors"
)

type TaskRunner struct {
	runningMutex *sync.Mutex
	runningCond  sync.Cond
	running      bool

	tasks     syncx.Map[port.TaskID, *port.TaskState]
	handlers  syncx.Map[port.TaskType, port.TaskHandler]
	semaphore chan struct{}

	cleanupDelay    time.Duration
	cleanupInterval time.Duration
}

// Run implements port.TaskRunner.
func (r *TaskRunner) Run(ctx context.Context) error {
	r.runningMutex.Lock()
	r.running = true
	r.runningCond.Broadcast()
	r.runningMutex.Unlock()

	go func() {
		ticker := time.NewTicker(r.cleanupInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				slog.DebugContext(ctx, "running task cleaner")

				r.tasks.Range(func(id port.TaskID, state *port.TaskState) bool {
					if state.FinishedAt.IsZero() || time.Now().After(state.FinishedAt.Add(r.cleanupDelay)) {
						return true
					}

					slog.DebugContext(ctx, "deleting expired task", slog.String("taskID", string(id)))

					r.tasks.Delete(id)

					return true
				})
			}
		}
	}()

	<-ctx.Done()

	if err := ctx.Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// List implements port.TaskRunner.
func (r *TaskRunner) List(ctx context.Context) ([]port.TaskStateHeader, error) {
	headers := make([]port.TaskStateHeader, 0)
	r.tasks.Range(func(id port.TaskID, state *port.TaskState) bool {
		headers = append(headers, state.TaskStateHeader)
		return true
	})
	return headers, nil
}

// Register implements port.TaskRunner.
func (r *TaskRunner) Register(taskType port.TaskType, handler port.TaskHandler) {
	r.handlers.Store(taskType, handler)
}

// Schedule implements port.TaskRunner.
func (r *TaskRunner) Schedule(ctx context.Context, task port.Task) error {
	taskID := task.ID()

	ctx = log.WithAttrs(ctx,
		slog.String("taskID", string(taskID)),
		slog.String("taskType", string(task.Type())),
	)

	r.updateState(taskID, func(s *port.TaskState) {
		s.ID = taskID
		s.ScheduledAt = time.Now()
		s.Status = port.TaskStatusPending
	})

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err, ok := recovered.(error)
				if !ok {
					err = errors.Errorf("%+v", recovered)
				}

				slog.ErrorContext(ctx, "recovered panic while running task", slog.Any("error", errors.WithStack(err)))

				r.updateState(taskID, func(s *port.TaskState) {
					s.Error = errors.WithStack(err)
					s.Status = port.TaskStatusFailed
				})
			}
		}()

		r.runningMutex.Lock()
		if !r.running {
			r.runningCond.Wait()
		}
		r.runningMutex.Unlock()

		r.semaphore <- struct{}{}
		defer func() {
			<-r.semaphore
		}()

		handler, exists := r.handlers.Load(task.Type())
		if !exists {
			r.updateState(taskID, func(s *port.TaskState) {
				s.Error = errors.Errorf("no handler registered for task type '%s'", task.Type())
				s.Status = port.TaskStatusFailed
			})

			return
		}

		r.updateState(taskID, func(s *port.TaskState) {
			s.Status = port.TaskStatusRunning
		})

		events := make(chan port.TaskEvent)
		defer close(events)

		go func() {
			for e := range events {
				r.updateState(taskID, func(s *port.TaskState) {
					if e.Progress != nil {
						s.Progress = float32(max(min(*e.Progress, 1), 0))
					}
					if e.Message != nil {
						s.Message = *e.Message
					}
				})
			}
		}()

		slog.DebugContext(ctx, "executing task")

		start := time.Now()

		if err := handler.Handle(ctx, task, events); err != nil {
			err = errors.WithStack(err)
			slog.ErrorContext(ctx, "task failed", slog.Any("error", err))

			r.updateState(taskID, func(s *port.TaskState) {
				s.Error = err
				s.Status = port.TaskStatusFailed
				s.FinishedAt = time.Now()
			})
			return
		}

		slog.DebugContext(ctx, "task finished", slog.Duration("duration", time.Now().Sub(start)))

		r.updateState(taskID, func(s *port.TaskState) {
			s.Status = port.TaskStatusSucceeded
			s.FinishedAt = time.Now()
		})
	}()
	return nil
}

func (r *TaskRunner) updateState(taskID port.TaskID, fn func(s *port.TaskState)) {
	state, _ := r.tasks.LoadOrStore(taskID, &port.TaskState{
		TaskStateHeader: port.TaskStateHeader{
			ID: taskID,
		},
	})

	fn(state)

	r.tasks.Store(taskID, state)
}

// State implements port.TaskRunner.
func (r *TaskRunner) State(ctx context.Context, id port.TaskID) (*port.TaskState, error) {
	state, exists := r.tasks.Load(id)
	if !exists {
		return nil, errors.WithStack(port.ErrNotFound)
	}

	return func(s port.TaskState) *port.TaskState {
		return &s
	}(*state), nil
}

func NewTaskRunner(parallelism int, cleanupDelay time.Duration, cleanupInterval time.Duration) *TaskRunner {
	runningMutex := &sync.Mutex{}
	return &TaskRunner{
		runningMutex:    runningMutex,
		runningCond:     *sync.NewCond(runningMutex),
		running:         false,
		semaphore:       make(chan struct{}, parallelism),
		tasks:           syncx.Map[port.TaskID, *port.TaskState]{},
		handlers:        syncx.Map[port.TaskType, port.TaskHandler]{},
		cleanupDelay:    cleanupDelay,
		cleanupInterval: cleanupInterval,
	}
}

var _ port.TaskRunner = &TaskRunner{}
