package backup

import "github.com/bornholm/corpus/internal/core/model"

const TaskTypeRestoreBackup model.TaskType = "restore_backup"

type RestoreBackupTask struct {
	id    model.TaskID
	owner model.User
	path  string
}

// Owner implements [model.Task].
func (i *RestoreBackupTask) Owner() model.User {
	return i.owner
}

// ID implements port.Task.
func (i *RestoreBackupTask) ID() model.TaskID {
	return i.id
}

// Type implements port.Task.
func (i *RestoreBackupTask) Type() model.TaskType {
	return TaskTypeRestoreBackup
}

func NewRestoreBackupTask(owner model.User, path string) *RestoreBackupTask {
	return &RestoreBackupTask{
		id:    model.NewTaskID(),
		owner: owner,
		path:  path,
	}
}

var _ model.Task = &RestoreBackupTask{}
