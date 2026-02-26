package document

import (
	"encoding/json"
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

const TaskTypeIndexFile model.TaskType = "index_file"

type IndexFileTask struct {
	id           model.TaskID
	owner        model.User
	path         string
	originalName string
	etag         string
	source       *url.URL
	// Names of the collection to associate with the document
	collections []model.CollectionID
}

type indexTaskPayload struct {
	Path         string               `json:"path"`
	OriginalName string               `json:"originalName"`
	Etag         string               `json:"etag"`
	Source       string               `json:"source"`
	Collections  []model.CollectionID `json:"collections"`
}

// MarshalJSON implements [model.Task].
func (i *IndexFileTask) MarshalJSON() ([]byte, error) {
	var sourceStr string
	if i.source != nil {
		sourceStr = i.source.String()
	}

	payload := indexTaskPayload{
		Path:         i.path,
		OriginalName: i.originalName,
		Etag:         i.etag,
		Source:       sourceStr,
		Collections:  i.collections,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

// UnmarshalJSON implements [model.Task].
func (i *IndexFileTask) UnmarshalJSON(data []byte) error {
	var payload indexTaskPayload

	if err := json.Unmarshal(data, &payload); err != nil {
		return errors.WithStack(err)
	}

	i.collections = payload.Collections
	i.etag = payload.Etag
	i.originalName = payload.OriginalName
	i.path = payload.Path

	source, err := url.Parse(payload.Source)
	if err != nil {
		return errors.WithStack(err)
	}

	i.source = source

	return nil
}

func NewIndexFileTask(owner model.User, path string, originalName string, etag string, source *url.URL, collections []model.CollectionID) *IndexFileTask {
	return &IndexFileTask{
		id:           model.NewTaskID(),
		owner:        owner,
		path:         path,
		originalName: originalName,
		etag:         etag,
		source:       source,
		collections:  collections,
	}
}

// ID implements model.Task.
func (i *IndexFileTask) ID() model.TaskID {
	return i.id
}

// Type implements model.Task.
func (i *IndexFileTask) Type() model.TaskType {
	return TaskTypeIndexFile
}

// Owner implements [model.Task].
func (i *IndexFileTask) Owner() model.User {
	return i.owner
}

var _ model.Task = &IndexFileTask{}
