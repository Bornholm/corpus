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

type TaskManager struct {
	runningMutex *sync.Mutex
	runningCond  sync.Cond
	running      bool

	tasks     syncx.Map[port.TaskID, *port.TaskState]
	handlers  syncx.Map[port.TaskType, port.TaskHandler]
	semaphore chan struct{}

	cleanupDelay    time.Duration
	cleanupInterval time.Duration
}

// Run implements port.TaskManager.
func (m *TaskManager) Run(ctx context.Context) error {
	m.runningMutex.Lock()
	m.running = true
	m.runningCond.Broadcast()
	m.runningMutex.Unlock()

	go func() {
		ticker := time.NewTicker(m.cleanupInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				slog.DebugContext(ctx, "running task cleaner")

				m.tasks.Range(func(id port.TaskID, state *port.TaskState) bool {
					if state.FinishedAt.IsZero() || time.Now().After(state.FinishedAt.Add(m.cleanupDelay)) {
						return true
					}

					slog.DebugContext(ctx, "deleting expired task", slog.String("taskID", string(id)))

					m.tasks.Delete(id)

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

// List implements port.TaskManager.
func (m *TaskManager) List(ctx context.Context) ([]port.TaskID, error) {
	tasks := make([]port.TaskID, 0)
	m.tasks.Range(func(id port.TaskID, _ *port.TaskState) bool {
		tasks = append(tasks, id)
		return true
	})
	return tasks, nil
}

// Register implements port.TaskManager.
func (m *TaskManager) Register(taskType port.TaskType, handler port.TaskHandler) {
	m.handlers.Store(taskType, handler)
}

// Schedule implements port.TaskManager.
func (m *TaskManager) Schedule(ctx context.Context, task port.Task) error {
	taskID := task.ID()

	ctx = log.WithAttrs(ctx,
		slog.String("taskID", string(taskID)),
		slog.String("taskType", string(task.Type())),
	)

	var stateMutex sync.Mutex
	updateState := func(fn func(s *port.TaskState)) {
		stateMutex.Lock()
		defer stateMutex.Unlock()

		state, ok := m.tasks.LoadOrStore(taskID, &port.TaskState{})
		if !ok {
			return
		}

		fn(state)

		m.tasks.Store(taskID, state)
	}

	updateState(func(s *port.TaskState) {
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
				updateState(func(s *port.TaskState) {
					s.Error = errors.WithStack(err)
					s.Status = port.TaskStatusFailed
				})
			}
		}()

		m.runningMutex.Lock()
		if !m.running {
			m.runningCond.Wait()
		}
		m.runningMutex.Unlock()

		m.semaphore <- struct{}{}
		defer func() {
			<-m.semaphore
		}()

		handler, exists := m.handlers.Load(task.Type())
		if !exists {
			updateState(func(s *port.TaskState) {
				s.Error = errors.Errorf("no handler registered for task type '%s'", task.Type())
				s.Status = port.TaskStatusFailed
			})

			return
		}

		updateState(func(s *port.TaskState) {
			s.Status = port.TaskStatusRunning
		})

		progress := make(chan float64)
		defer close(progress)

		go func() {
			for p := range progress {
				updateState(func(s *port.TaskState) {
					s.Progress = max(min(p, 100), 0)
				})
			}
		}()

		slog.DebugContext(ctx, "executing task")

		if err := handler.Handle(ctx, task, progress); err != nil {
			updateState(func(s *port.TaskState) {
				s.Error = errors.WithStack(err)
				s.Status = port.TaskStatusFailed
				s.FinishedAt = time.Now()
			})
			return
		}

		updateState(func(s *port.TaskState) {
			s.Status = port.TaskStatusSucceeded
			s.FinishedAt = time.Now()
		})
	}()
	return nil
}

// State implements port.TaskManager.
func (m *TaskManager) State(ctx context.Context, id port.TaskID) (*port.TaskState, error) {
	state, exists := m.tasks.Load(id)
	if !exists {
		return nil, errors.WithStack(port.ErrNotFound)
	}

	return func(s port.TaskState) *port.TaskState {
		return &s
	}(*state), nil
}

func NewTaskManager(parallelism int, cleanupDelay time.Duration, cleanupInterval time.Duration) *TaskManager {
	runningMutex := &sync.Mutex{}
	return &TaskManager{
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

var _ port.TaskManager = &TaskManager{}
