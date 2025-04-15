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
}

type Task interface {
	ID() TaskID
	Type() TaskType
}

type TaskHandler interface {
	Handle(ctx context.Context, task Task, progress chan float32) error
}

type TaskHandlerFunc func(ctx context.Context, task Task, progress chan float32) error

func (f TaskHandlerFunc) Handle(ctx context.Context, task Task, progress chan float32) error {
	return f(ctx, task, progress)
}

type TaskManager interface {
	Schedule(ctx context.Context, task Task) error
	State(ctx context.Context, id TaskID) (*TaskState, error)
	List(ctx context.Context) ([]TaskStateHeader, error)
	Register(taskType TaskType, handler TaskHandler)
	Run(ctx context.Context) error
}
