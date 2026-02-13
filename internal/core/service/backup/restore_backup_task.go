package backup

import (
	"encoding/json"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

const TaskTypeRestoreBackup model.TaskType = "restore_backup"

type restoreBackupTaskPayload struct {
	Path string `json:"path"`
}

type RestoreBackupTask struct {
	id    model.TaskID
	owner model.User
	path  string
}

// MarshalJSON implements [model.Task].
func (i *RestoreBackupTask) MarshalJSON() ([]byte, error) {
	payload := restoreBackupTaskPayload{
		Path: i.path,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

// UnmarshalJSON implements [model.Task].
func (i *RestoreBackupTask) UnmarshalJSON(data []byte) error {
	var payload restoreBackupTaskPayload

	if err := json.Unmarshal(data, &payload); err != nil {
		return errors.WithStack(err)
	}

	i.path = payload.Path

	return nil
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
