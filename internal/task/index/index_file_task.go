package index

import (
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
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
