package document

import (
	"encoding/json"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

const TaskTypeReindexCollection model.TaskType = "reindex_collection"

type reindexCollectionTaskPayload struct {
	CollectionID model.CollectionID `json:"collectionID"`
}

type ReindexCollectionTask struct {
	id           model.TaskID
	owner        model.User
	collectionID model.CollectionID
}

// MarshalJSON implements [model.Task].
func (t *ReindexCollectionTask) MarshalJSON() ([]byte, error) {
	payload := reindexCollectionTaskPayload{
		CollectionID: t.collectionID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

// UnmarshalJSON implements [model.Task].
func (t *ReindexCollectionTask) UnmarshalJSON(data []byte) error {
	var payload reindexCollectionTaskPayload

	if err := json.Unmarshal(data, &payload); err != nil {
		return errors.WithStack(err)
	}

	t.collectionID = payload.CollectionID

	return nil
}

// Owner implements [model.Task].
func (t *ReindexCollectionTask) Owner() model.User {
	return t.owner
}

// ID implements port.Task.
func (t *ReindexCollectionTask) ID() model.TaskID {
	return t.id
}

// Type implements port.Task.
func (t *ReindexCollectionTask) Type() model.TaskType {
	return TaskTypeReindexCollection
}

// CollectionID returns the collection ID to reindex.
func (t *ReindexCollectionTask) CollectionID() model.CollectionID {
	return t.collectionID
}

func NewReindexCollectionTask(owner model.User, collectionID model.CollectionID) *ReindexCollectionTask {
	return &ReindexCollectionTask{
		id:           model.NewTaskID(),
		owner:        owner,
		collectionID: collectionID,
	}
}

var _ model.Task = &ReindexCollectionTask{}
