package corpus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"time"

	bleve "github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	bleveAdapter "github.com/bornholm/corpus/pkg/adapter/bleve"
	gormAdapter "github.com/bornholm/corpus/pkg/adapter/gorm"
	memoryAdapter "github.com/bornholm/corpus/pkg/adapter/memory"
	"github.com/bornholm/corpus/pkg/adapter/pipeline"
	sqlitevecAdapter "github.com/bornholm/corpus/pkg/adapter/sqlitevec"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/corpus/internal/core/service"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/ncruces/go-sqlite3"
	gormlite "github.com/ncruces/go-sqlite3/gormlite"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	// Register sqlite-vec extension
	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

// Corpus is the main embedded corpus instance.
type Corpus struct {
	documentManager *service.DocumentManager
	taskRunner      port.TaskRunner
	userStore       port.UserStore
	systemUser      model.User
}

// New creates a new embedded Corpus instance.
//
// At minimum, either WithStoragePath (for auto-composition) or
// explicit WithIndex + WithDocumentStore + WithUserStore must be provided.
func New(ctx context.Context, funcs ...OptionFunc) (*Corpus, error) {
	opts := defaultOptions()
	for _, fn := range funcs {
		fn(opts)
	}

	docStore := opts.documentStore
	userStore := opts.userStore
	idx := opts.index
	taskRunner := opts.taskRunner

	// Auto-compose missing components
	if docStore == nil || userStore == nil {
		if opts.storagePath == "" && opts.databaseDSN == "" {
			return nil, errors.New("corpus: WithStoragePath or WithDatabaseDSN is required when no DocumentStore is provided")
		}

		databaseDSN := opts.databaseDSN
		if databaseDSN == "" {
			databaseDSN = filepath.Join(opts.storagePath, "data.sqlite")
		}

		db, err := newGormDB(databaseDSN)
		if err != nil {
			return nil, errors.Wrap(err, "could not open database")
		}

		store := gormAdapter.NewStore(db)
		if docStore == nil {
			docStore = store
		}
		if userStore == nil {
			userStore = store
		}
	}

	if idx == nil {
		if opts.storagePath == "" && (opts.bleveDSN == "" || opts.sqliteVecDSN == "") {
			return nil, errors.New("corpus: WithStoragePath or both WithBleveDSN and WithSQLiteVecDSN are required when no Index is provided")
		}

		bleveDSN := opts.bleveDSN
		if bleveDSN == "" {
			bleveDSN = filepath.Join(opts.storagePath, "index.bleve")
		}

		bleveIdx, err := newBleveIndex(ctx, bleveDSN)
		if err != nil {
			return nil, errors.Wrap(err, "could not open bleve index")
		}

		sqliteVecDSN := opts.sqliteVecDSN
		if sqliteVecDSN == "" {
			sqliteVecDSN = filepath.Join(opts.storagePath, "index.sqlite")
		}

		sqliteConn, err := sqlite3.Open(sqliteVecDSN)
		if err != nil {
			return nil, errors.Wrap(err, "could not open sqlitevec database")
		}

		sqliteVecIdx := sqlitevecAdapter.NewIndex(sqliteConn, opts.llmClient, opts.embeddingsModel, opts.maxIndexWords)

		weightedIndexes := pipeline.WeightedIndexes{
			pipeline.NewIdentifiedIndex("bleve", bleveIdx):      opts.bleveWeight,
			pipeline.NewIdentifiedIndex("sqlitevec", sqliteVecIdx): opts.sqliteVecWeight,
		}

		pipelineOpts := []pipeline.OptionFunc{}
		if !opts.disableHyDE && opts.llmClient != nil {
			pipelineOpts = append(pipelineOpts,
				pipeline.WithQueryTransformers(
					pipeline.NewHyDEQueryTransformer(opts.llmClient, docStore),
				),
			)
		}

		resultsTransformers := []pipeline.ResultsTransformer{
			pipeline.NewDuplicateContentResultsTransformer(docStore),
		}
		if !opts.disableJudge && opts.llmClient != nil {
			resultsTransformers = append(resultsTransformers,
				pipeline.NewJudgeResultsTransformer(opts.llmClient, docStore, opts.maxTotalWords),
			)
		}
		pipelineOpts = append(pipelineOpts,
			pipeline.WithResultsTransformers(resultsTransformers...),
		)

		idx = pipeline.NewIndex(weightedIndexes, pipelineOpts...)
	}

	if taskRunner == nil {
		taskRunner = memoryAdapter.NewTaskRunner(
			opts.taskParallelism,
			60*time.Minute,
			10*time.Minute,
		)
	}

	dmOpts := []service.DocumentManagerOptionFunc{}
	if opts.fileConverter != nil {
		dmOpts = append(dmOpts, service.WithDocumentManagerFileConverter(opts.fileConverter))
	}

	documentManager := service.NewDocumentManager(docStore, idx, taskRunner, opts.llmClient, dmOpts...)

	// Register task handlers
	indexFileHandler := documentTask.NewIndexFileHandler(
		userStore, docStore, opts.fileConverter, idx, opts.maxWordsPerSection,
	)
	taskRunner.RegisterTask(documentTask.TaskTypeIndexFile, indexFileHandler)

	cleanupHandler := documentTask.NewCleanupHandler(idx, docStore)
	taskRunner.RegisterTask(documentTask.TaskTypeCleanup, cleanupHandler)

	reindexCollectionHandler := documentTask.NewReindexHandler(docStore, idx, opts.maxWordsPerSection)
	taskRunner.RegisterTask(documentTask.TaskTypeReindexCollection, reindexCollectionHandler)

	// Start task runner
	go func() {
		if err := taskRunner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.ErrorContext(ctx, "corpus task runner stopped", slog.Any("error", err))
		}
	}()

	// Create or retrieve the embedded system user
	systemUser, err := userStore.FindOrCreateUser(ctx, "embedded", "system")
	if err != nil {
		return nil, errors.Wrap(err, "could not find or create system user")
	}

	return &Corpus{
		documentManager: documentManager,
		taskRunner:      taskRunner,
		userStore:       userStore,
		systemUser:      systemUser,
	}, nil
}

// IndexFile indexes a file into the given collection.
// Returns a TaskID that can be used to track progress via GetTaskState.
func (c *Corpus) IndexFile(ctx context.Context, collectionID model.CollectionID, filename string, r io.Reader, funcs ...IndexFileOptionFunc) (model.TaskID, error) {
	opts := &IndexFileOptions{
		Collections: []model.CollectionID{collectionID},
	}
	for _, fn := range funcs {
		fn(opts)
	}

	dmOpts := []service.DocumentManagerIndexFileOptionFunc{
		service.WithDocumentManagerIndexFileCollections(opts.Collections...),
	}
	if opts.Source != nil {
		dmOpts = append(dmOpts, service.WithDocumentManagerIndexFileSource(opts.Source))
	}
	if opts.ETag != "" {
		dmOpts = append(dmOpts, service.WithDocumentManagerIndexFileETag(opts.ETag))
	}

	taskID, err := c.documentManager.IndexFile(ctx, c.systemUser, filename, r, dmOpts...)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return taskID, nil
}

// Search performs a semantic search across the corpus.
func (c *Corpus) Search(ctx context.Context, query string, funcs ...SearchOptionFunc) ([]*port.IndexSearchResult, error) {
	opts := &SearchOptions{
		MaxResults: 5,
	}
	for _, fn := range funcs {
		fn(opts)
	}

	dmOpts := []service.DocumentManagerSearchOptionFunc{
		service.WithDocumentManagerSearchMaxResults(opts.MaxResults),
	}
	if len(opts.Collections) > 0 {
		dmOpts = append(dmOpts, service.WithDocumentManagerSearchCollections(opts.Collections...))
	}

	results, err := c.documentManager.Search(ctx, query, dmOpts...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return results, nil
}

// Ask generates a LLM response based on search results.
// Returns the answer and a map of section IDs to their content.
func (c *Corpus) Ask(ctx context.Context, query string, results []*port.IndexSearchResult, funcs ...AskOptionFunc) (string, map[model.SectionID]string, error) {
	opts := &AskOptions{}
	for _, fn := range funcs {
		fn(opts)
	}

	dmOpts := []service.DocumentManagerAskOptionFunc{}
	if opts.SystemPromptTemplate != "" {
		dmOpts = append(dmOpts, service.WithAskSystemPromptTemplate(opts.SystemPromptTemplate))
	}

	answer, sections, err := c.documentManager.Ask(ctx, query, results, dmOpts...)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	return answer, sections, nil
}

// GetTaskState returns the current state of an indexing task.
func (c *Corpus) GetTaskState(ctx context.Context, id model.TaskID) (*port.TaskState, error) {
	state, err := c.taskRunner.GetTaskState(ctx, id)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return state, nil
}

// CreateCollection creates a new collection and returns its ID.
func (c *Corpus) CreateCollection(ctx context.Context, label string) (model.CollectionID, error) {
	coll, err := c.documentManager.CreateCollection(ctx, c.systemUser.ID(), label)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return coll.ID(), nil
}

// DeleteBySource removes all documents and index entries for the given source URL.
func (c *Corpus) DeleteBySource(ctx context.Context, source *url.URL) error {
	if err := c.documentManager.DeleteDocumentBySource(ctx, c.systemUser.ID(), source); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// DocumentManager returns the underlying service.DocumentManager for advanced usage.
func (c *Corpus) DocumentManager() *service.DocumentManager {
	return c.documentManager
}

// newGormDB opens a SQLite database with WAL mode and returns a GORM DB.
func newGormDB(dsn string) (*gorm.DB, error) {
	dialector := gormlite.Open(dsn)

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.New(
			log.New(os.Stderr, "\r\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormlogger.Error,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	internalDB, err := db.DB()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	internalDB.SetMaxOpenConns(1)

	if err := db.Exec("PRAGMA journal_mode=wal; PRAGMA foreign_keys=on; PRAGMA busy_timeout=5000").Error; err != nil {
		return nil, errors.WithStack(err)
	}

	return db, nil
}

const mappingHashFilename = ".mapping_hash"

func mappingHash(m *mapping.IndexMappingImpl) (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", errors.Wrap(err, "could not marshal mapping")
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// newBleveIndex opens or creates a Bleve index at the given path.
// If the mapping has changed since last open, the index is recreated.
func newBleveIndex(ctx context.Context, indexPath string) (port.Index, error) {
	stat, err := os.Stat(indexPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.WithStack(err)
	}

	var bleveIdx bleve.Index

	if stat == nil {
		m := bleveAdapter.IndexMapping()

		bleveIdx, err = bleve.New(indexPath, m)
		if err != nil {
			return nil, errors.Wrap(err, "could not create bleve index")
		}

		hash, err := mappingHash(m)
		if err != nil {
			slog.WarnContext(ctx, "could not compute mapping hash", slog.Any("error", err))
		} else {
			if err := os.WriteFile(filepath.Join(indexPath, mappingHashFilename), []byte(hash), 0600); err != nil {
				slog.WarnContext(ctx, "could not store mapping hash", slog.Any("error", err))
			}
		}
	} else {
		currentMapping := bleveAdapter.IndexMapping()
		currentHash, err := mappingHash(currentMapping)
		if err != nil {
			slog.WarnContext(ctx, "could not compute current mapping hash", slog.Any("error", err))
		} else {
			hashFile := filepath.Join(indexPath, mappingHashFilename)
			storedHashBytes, err := os.ReadFile(hashFile)
			storedHash := string(storedHashBytes)
			if err != nil {
				slog.WarnContext(ctx, "could not read stored mapping hash", slog.Any("error", err))
			}

			if storedHash != currentHash {
				slog.InfoContext(ctx, "bleve index mapping has changed, recreating index",
					slog.String("path", indexPath))

				if err := os.RemoveAll(indexPath); err != nil {
					slog.WarnContext(ctx, "could not delete old bleve index, opening as-is", slog.Any("error", err))
					bleveIdx, err = bleve.Open(indexPath)
					if err != nil {
						return nil, errors.Wrap(err, "could not open bleve index")
					}
				} else {
					bleveIdx, err = bleve.New(indexPath, currentMapping)
					if err != nil {
						return nil, errors.Wrap(err, "could not create new bleve index")
					}
				}

				if err := os.WriteFile(hashFile, []byte(currentHash), 0600); err != nil {
					slog.WarnContext(ctx, "could not store mapping hash", slog.Any("error", err))
				}

				return bleveAdapter.NewIndex(bleveIdx), nil
			}
		}

		bleveIdx, err = bleve.Open(indexPath)
		if err != nil {
			return nil, errors.Wrap(err, "could not open bleve index")
		}
	}

	return bleveAdapter.NewIndex(bleveIdx), nil
}
