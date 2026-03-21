package document

import (
	"github.com/bornholm/corpus/pkg/model"
)

const TaskTypeReindexBleve model.TaskType = "reindex_bleve"

type ReindexBleveTask struct {
	id    model.TaskID
	owner model.User
}

// MarshalJSON implements [model.Task].
func (t *ReindexBleveTask) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

// UnmarshalJSON implements [model.Task].
func (t *ReindexBleveTask) UnmarshalJSON(data []byte) error {
	return nil
}

// Owner implements [model.Task].
func (t *ReindexBleveTask) Owner() model.User {
	return t.owner
}

// ID implements port.Task.
func (t *ReindexBleveTask) ID() model.TaskID {
	return t.id
}

// Type implements port.Task.
func (t *ReindexBleveTask) Type() model.TaskType {
	return TaskTypeReindexBleve
}

func NewReindexBleveTask(owner model.User) *ReindexBleveTask {
	return &ReindexBleveTask{
		id:    model.NewTaskID(),
		owner: owner,
	}
}

var _ model.Task = &ReindexBleveTask{}
