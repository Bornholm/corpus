package setup

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/pkg/errors"
)

// reindexBleveHandler is a TaskHandler that resolves the bleve index lazily
// to avoid a circular initialization deadlock:
//
//	getBleveIndexFromConfig (sync.Once) → getTaskRunner → setupTaskHandlers
//	→ getReindexBleveTaskHandler → getBleveIndexFromConfig (deadlock)
type reindexBleveHandler struct {
	conf              *config.Config
	documentStore     port.DocumentStore
	maxWordPerSection int
}

func (h *reindexBleveHandler) Handle(ctx context.Context, task model.Task, events chan port.TaskEvent) error {
	// Use the cached index reference to avoid re-opening and potential deadlocks.
	// The index was already opened during startup and stored in cachedBleveIndex.
	if cachedBleveIndex == nil {
		// Fallback: if somehow the cached reference is nil, try to get it.
		// This shouldn't happen if initialization completed properly.
		slog.WarnContext(ctx, "reindexBleveHandler: cached index is nil, re-opening")
		var err error
		cachedBleveIndex, err = getBleveIndexFromConfig(ctx, h.conf)
		if err != nil {
			return errors.Wrap(err, "could not get bleve index")
		}
	}

	delegate := documentTask.NewReindexHandler(h.documentStore, cachedBleveIndex, h.maxWordPerSection)

	return delegate.Handle(ctx, task, events)
}

var _ port.TaskHandler = &reindexBleveHandler{}

var getReindexBleveTaskHandler = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (*reindexBleveHandler, error) {
	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	// Do NOT call getBleveIndexFromConfig here — would deadlock if mapping has changed.
	return &reindexBleveHandler{
		conf:              conf,
		documentStore:     documentStore,
		maxWordPerSection: conf.LLM.Index.MaxWords,
	}, nil
})
