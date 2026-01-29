package document

import "github.com/bornholm/corpus/internal/core/model"

const TaskTypeCleanup model.TaskType = "cleanup"

type CleanupTask struct {
	id          model.TaskID
	owner       model.User
	collections []model.CollectionID
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
