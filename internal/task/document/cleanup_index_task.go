package document

import (
	"encoding/json"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

const TaskTypeCleanup model.TaskType = "cleanup"

type cleanupTaskPayload struct {
	Collections []model.CollectionID `json:"collections"`
}
type CleanupTask struct {
	id          model.TaskID
	owner       model.User
	collections []model.CollectionID
}

// MarshalJSON implements [model.Task].
func (t *CleanupTask) MarshalJSON() ([]byte, error) {
	payload := cleanupTaskPayload{
		Collections: t.collections,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

// UnmarshalJSON implements [model.Task].
func (t *CleanupTask) UnmarshalJSON(data []byte) error {
	var payload cleanupTaskPayload

	if err := json.Unmarshal(data, &payload); err != nil {
		return errors.WithStack(err)
	}

	t.collections = payload.Collections

	return nil
}

// Owner implements [model.Task].
func (t *CleanupTask) Owner() model.User {
	return t.owner
}

// ID implements port.Task.
func (t *CleanupTask) ID() model.TaskID {
	return t.id
}

// Type implements port.Task.
func (i *CleanupTask) Type() model.TaskType {
	return TaskTypeCleanup
}

func NewCleanupTask(owner model.User, collections []model.CollectionID) *CleanupTask {
	return &CleanupTask{
		id:          model.NewTaskID(),
		owner:       owner,
		collections: collections,
	}
}

var _ model.Task = &CleanupTask{}
