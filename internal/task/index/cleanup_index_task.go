package index

import "github.com/bornholm/corpus/internal/core/model"

const TaskTypeCleanupIndex model.TaskType = "cleanup_index"

type CleanupIndexTask struct {
	id          model.TaskID
	owner       model.User
	collections []model.CollectionID
}

// Owner implements [model.Task].
func (t *CleanupIndexTask) Owner() model.User {
	return t.owner
}

// ID implements port.Task.
func (t *CleanupIndexTask) ID() model.TaskID {
	return t.id
}

// Type implements port.Task.
func (i *CleanupIndexTask) Type() model.TaskType {
	return TaskTypeCleanupIndex
}

func NewCleanupIndexTask(owner model.User, collections []model.CollectionID) *CleanupIndexTask {
	return &CleanupIndexTask{
		id:          model.NewTaskID(),
		owner:       owner,
		collections: collections,
	}
}

var _ model.Task = &CleanupIndexTask{}
