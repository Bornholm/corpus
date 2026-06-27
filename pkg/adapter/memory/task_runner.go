package memory

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bornholm/corpus/pkg/adapter/memory/syncx"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

type taskEntry struct {
	Task  model.Task
	State port.TaskState
}

type queuedTask struct {
	task   model.Task
	ctx    context.Context
	cancel context.CancelFunc
}

type TaskRunner struct {
	runningMutex *sync.Mutex
	runningCond  sync.Cond
	running      bool

	tasks      syncx.Map[model.TaskID, taskEntry]
	stateMutex sync.Mutex

	handlers  syncx.Map[model.TaskType, port.TaskHandler]
	queue     chan queuedTask
	errOnFull bool

	cancelFuncs syncx.Map[model.TaskID, context.CancelFunc]

	parallelism     int
	cleanupDelay    time.Duration
	cleanupInterval time.Duration
}

// CancelTask implements [port.TaskRunner].
func (r *TaskRunner) CancelTask(ctx context.Context, id model.TaskID) error {
	entry, exists := r.tasks.Load(id)
	if !exists {
		return errors.WithStack(port.ErrNotFound)
	}

	if entry.State.Status != port.TaskStatusPending && entry.State.Status != port.TaskStatusRunning {
		return errors.WithStack(port.ErrCanceled)
	}

	cancelFn, exists := r.cancelFuncs.Load(id)
	if !exists {
		return errors.WithStack(port.ErrCanceled)
	}

	cancelFn()

	r.updateState(entry.Task, func(s *port.TaskState) {
		s.Error = errors.WithStack(port.ErrCanceled)
		s.Status = port.TaskStatusFailed
		s.FinishedAt = time.Now()
	})

	r.cancelFuncs.Delete(id)

	return nil
}

// GetTask implements [port.TaskRunner].
func (r *TaskRunner) GetTask(ctx context.Context, id model.TaskID) (model.Task, error) {
	entry, exists := r.tasks.Load(id)
	if !exists {
		return nil, errors.WithStack(port.ErrNotFound)
	}
	return entry.Task, nil
}

// Run implements port.TaskRunner.
func (r *TaskRunner) Run(ctx context.Context) error {
	r.runningMutex.Lock()
	r.running = true
	r.runningCond.Broadcast()
	r.runningMutex.Unlock()

	// Start fixed worker pool
	var wg sync.WaitGroup
	for i := 0; i < r.parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.runningMutex.Lock()
			for !r.running {
				r.runningCond.Wait()
			}
			r.runningMutex.Unlock()

			for {
				select {
				case <-ctx.Done():
					return
				case qt, ok := <-r.queue:
					if !ok {
						return
					}
					r.executeTask(qt)
				}
			}
		}()
	}

	// Cleanup goroutine
	go func() {
		ticker := time.NewTicker(r.cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				slog.DebugContext(ctx, "running task cleaner")

				var idsToDelete []model.TaskID
				r.tasks.Range(func(id model.TaskID, entry taskEntry) bool {
					if entry.State.FinishedAt.IsZero() || !time.Now().After(entry.State.FinishedAt.Add(r.cleanupDelay)) {
						return true
					}
					idsToDelete = append(idsToDelete, id)
					return true
				})

				for _, id := range idsToDelete {
					slog.DebugContext(ctx, "deleting expired task", slog.String("taskID", string(id)))
					r.tasks.Delete(id)
					r.cancelFuncs.Delete(id)
				}
			}
		}
	}()

	<-ctx.Done()
	close(r.queue)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (r *TaskRunner) executeTask(qt queuedTask) {
	task := qt.task
	taskCtx := qt.ctx
	taskID := task.ID()

	ctx := slogx.WithAttrs(taskCtx,
		slog.String("taskID", string(taskID)),
		slog.String("taskType", string(task.Type())),
	)

	defer func() {
		r.cancelFuncs.Delete(taskID)

		if recovered := recover(); recovered != nil {
			err, ok := recovered.(error)
			if !ok {
				err = errors.Errorf("%+v", recovered)
			}
			slog.ErrorContext(ctx, "recovered panic while running task", slog.Any("error", errors.WithStack(err)))
			r.updateState(task, func(s *port.TaskState) {
				s.Error = errors.WithStack(err)
				s.Status = port.TaskStatusFailed
				s.FinishedAt = time.Now()
			})
		}
	}()

	handler, exists := r.handlers.Load(task.Type())
	if !exists {
		r.updateState(task, func(s *port.TaskState) {
			s.Error = errors.Errorf("no handler registered for task type '%s'", task.Type())
			s.Status = port.TaskStatusFailed
			s.FinishedAt = time.Now()
		})
		return
	}

	r.updateState(task, func(s *port.TaskState) {
		s.Status = port.TaskStatusRunning
	})

	events := make(chan port.TaskEvent, 100)

	var eventsWg sync.WaitGroup
	eventsWg.Add(1)
	go func() {
		defer eventsWg.Done()
		for e := range events {
			r.updateState(task, func(s *port.TaskState) {
				if e.Progress != nil {
					s.Progress = float32(max(min(*e.Progress, 1), 0))
				}
				if e.Message != nil {
					s.Message = *e.Message
				}
			})
		}
	}()

	start := time.Now()

	err := handler.Handle(taskCtx, task, events)

	if errors.Is(err, port.ErrCanceled) {
		slog.DebugContext(ctx, "task was canceled")
		r.updateState(task, func(s *port.TaskState) {
			s.Error = errors.WithStack(port.ErrCanceled)
			s.Status = port.TaskStatusFailed
			s.FinishedAt = time.Now()
		})
		close(events)
		eventsWg.Wait()
		return
	}

	close(events)
	eventsWg.Wait()

	if err != nil {
		err = errors.WithStack(err)
		slog.ErrorContext(ctx, "task failed", slog.Any("error", err))
		r.updateState(task, func(s *port.TaskState) {
			s.Error = err
			s.Status = port.TaskStatusFailed
			s.FinishedAt = time.Now()
		})
		return
	}

	slog.DebugContext(ctx, "task finished", slog.Duration("duration", time.Since(start)))
	r.updateState(task, func(s *port.TaskState) {
		s.Status = port.TaskStatusSucceeded
		s.FinishedAt = time.Now()
		s.Progress = 1
	})
}

// ListTasks implements port.TaskRunner.
func (r *TaskRunner) ListTasks(ctx context.Context) ([]port.TaskStateHeader, error) {
	headers := make([]port.TaskStateHeader, 0)
	r.tasks.Range(func(id model.TaskID, entry taskEntry) bool {
		headers = append(headers, entry.State.TaskStateHeader)
		return true
	})
	return headers, nil
}

// RegisterTask implements port.TaskRunner.
func (r *TaskRunner) RegisterTask(taskType model.TaskType, handler port.TaskHandler) {
	r.handlers.Store(taskType, handler)
}

// ScheduleTask implements port.TaskRunner.
func (r *TaskRunner) ScheduleTask(ctx context.Context, task model.Task) error {
	taskID := task.ID()

	ctx = slogx.WithAttrs(ctx,
		slog.String("taskID", string(taskID)),
		slog.String("taskType", string(task.Type())),
	)

	r.updateState(task, func(s *port.TaskState) {
		s.ID = taskID
		s.ScheduledAt = time.Now()
		s.Status = port.TaskStatusPending
		s.Type = task.Type()
	})

	taskCtx, cancelFn := context.WithCancel(context.Background())
	r.cancelFuncs.Store(taskID, cancelFn)

	qt := queuedTask{task: task, ctx: taskCtx, cancel: cancelFn}

	if r.errOnFull {
		select {
		case r.queue <- qt:
		default:
			cancelFn()
			r.cancelFuncs.Delete(taskID)
			r.updateState(task, func(s *port.TaskState) {
				s.Error = errors.WithStack(port.ErrQueueFull)
				s.Status = port.TaskStatusFailed
				s.FinishedAt = time.Now()
			})
			return errors.WithStack(port.ErrQueueFull)
		}
	} else {
		r.queue <- qt
	}

	return nil
}

func (r *TaskRunner) updateState(task model.Task, fn func(s *port.TaskState)) {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()

	entry, _ := r.tasks.LoadOrStore(task.ID(), taskEntry{
		Task: task,
		State: port.TaskState{
			TaskStateHeader: port.TaskStateHeader{
				ID: task.ID(),
			},
		},
	})

	fn(&entry.State)

	r.tasks.Store(task.ID(), entry)
}

// GetTaskState implements port.TaskRunner.
func (r *TaskRunner) GetTaskState(ctx context.Context, id model.TaskID) (*port.TaskState, error) {
	entry, exists := r.tasks.Load(id)
	if !exists {
		return nil, errors.WithStack(port.ErrNotFound)
	}
	return &entry.State, nil
}

func NewTaskRunner(parallelism int, cleanupDelay time.Duration, cleanupInterval time.Duration) *TaskRunner {
	return NewTaskRunnerWithQueue(parallelism, parallelism*4, false, cleanupDelay, cleanupInterval)
}

func NewTaskRunnerWithQueue(parallelism int, queueSize int, errOnFull bool, cleanupDelay time.Duration, cleanupInterval time.Duration) *TaskRunner {
	runningMutex := &sync.Mutex{}
	return &TaskRunner{
		runningMutex:    runningMutex,
		runningCond:     *sync.NewCond(runningMutex),
		running:         false,
		parallelism:     parallelism,
		queue:           make(chan queuedTask, queueSize),
		errOnFull:       errOnFull,
		tasks:           syncx.Map[model.TaskID, taskEntry]{},
		handlers:        syncx.Map[model.TaskType, port.TaskHandler]{},
		cancelFuncs:     syncx.Map[model.TaskID, context.CancelFunc]{},
		cleanupDelay:    cleanupDelay,
		cleanupInterval: cleanupInterval,
	}
}

var _ port.TaskRunner = &TaskRunner{}
