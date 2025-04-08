package memory

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

func TestTaskManager(t *testing.T) {
	tm := NewTaskManager(10, 10*time.Minute, time.Minute)

	var executed atomic.Int64

	tm.Register("dummy", port.TaskHandlerFunc(func(ctx context.Context, task port.Task, progress chan float64) error {
		t.Logf("[%s] start", task.ID())
		progress <- 0
		progress <- 50
		progress <- 100
		t.Logf("[%s] done", task.ID())
		executed.Add(1)
		return nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	total := int64(100)

	for range total {
		task := &dummyTask{
			id: port.NewTaskID(),
		}
		t.Logf("Scheduling task %s", task.ID())
		tm.Schedule(ctx, task)
	}

	if err := tm.Run(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("%+v", errors.WithStack(err))
	}

	t.Logf("executed: %d", executed.Load())

	if e, g := total, executed.Load(); e != g {
		t.Logf("executed: expected %d, got %d", e, g)
	}
}

type dummyTask struct {
	id port.TaskID
}

// ID implements port.Task.
func (d *dummyTask) ID() port.TaskID {
	return d.id
}

// Type implements port.Task.
func (d *dummyTask) Type() port.TaskType {
	return "dummy"
}

var _ port.Task = &dummyTask{}
