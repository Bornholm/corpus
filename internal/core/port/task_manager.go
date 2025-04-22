package port

import (
	"context"
	"time"

	"github.com/rs/xid"
)

type TaskID string

func NewTaskID() TaskID {
	return TaskID(xid.New().String())
}

type TaskType string

type TaskStatus string

const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"
)

type TaskStateHeader struct {
	ID          TaskID
	ScheduledAt time.Time
	Status      TaskStatus
}

type TaskState struct {
	TaskStateHeader
	FinishedAt time.Time
	Progress   float32
	Error      error
	Message    string
}

type TaskEvent struct {
	Message  *string
	Progress *float32
}

type TaskEventFunc func(p *TaskEvent)

func WithTaskMessage(message string) TaskEventFunc {
	return func(p *TaskEvent) {
		p.Message = &message
	}
}

func WithTaskProgress(progress float32) TaskEventFunc {
	return func(p *TaskEvent) {
		p.Progress = &progress
	}
}

func NewTaskEvent(funcs ...TaskEventFunc) TaskEvent {
	p := TaskEvent{}
	for _, fn := range funcs {
		fn(&p)
	}
	return p
}

type Task interface {
	ID() TaskID
	Type() TaskType
}

type TaskHandler interface {
	Handle(ctx context.Context, task Task, events chan TaskEvent) error
}

type TaskHandlerFunc func(ctx context.Context, task Task, events chan TaskEvent) error

func (f TaskHandlerFunc) Handle(ctx context.Context, task Task, events chan TaskEvent) error {
	return f(ctx, task, events)
}

type TaskManager interface {
	Schedule(ctx context.Context, task Task) error
	State(ctx context.Context, id TaskID) (*TaskState, error)
	List(ctx context.Context) ([]TaskStateHeader, error)
	Register(taskType TaskType, handler TaskHandler)
	Run(ctx context.Context) error
}
