package document

import (
	"encoding/json"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/pkg/errors"
)

const TaskTypeSyncFilesystemSource model.TaskType = "sync_filesystem_source"

type syncFilesystemSourcePayload struct {
	SourceID model.FilesystemSourceID `json:"source_id"`
}

type SyncFilesystemSourceTask struct {
	id       model.TaskID
	owner    model.User
	sourceID model.FilesystemSourceID
}

// MarshalJSON implements [model.Task].
func (t *SyncFilesystemSourceTask) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(syncFilesystemSourcePayload{SourceID: t.sourceID})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return data, nil
}

// UnmarshalJSON implements [model.Task].
func (t *SyncFilesystemSourceTask) UnmarshalJSON(data []byte) error {
	var payload syncFilesystemSourcePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return errors.WithStack(err)
	}
	t.sourceID = payload.SourceID
	return nil
}

// ID implements model.Task.
func (t *SyncFilesystemSourceTask) ID() model.TaskID { return t.id }

// Type implements model.Task.
func (t *SyncFilesystemSourceTask) Type() model.TaskType { return TaskTypeSyncFilesystemSource }

// Owner implements model.Task.
func (t *SyncFilesystemSourceTask) Owner() model.User { return t.owner }

func NewSyncFilesystemSourceTask(owner model.User, sourceID model.FilesystemSourceID) *SyncFilesystemSourceTask {
	return &SyncFilesystemSourceTask{
		id:       model.NewTaskID(),
		owner:    owner,
		sourceID: sourceID,
	}
}

var _ model.Task = &SyncFilesystemSourceTask{}
