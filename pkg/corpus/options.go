package corpus

import (
	"net/url"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/genai/llm"
)

type options struct {
	// High-level options (auto-composition)
	storagePath        string
	databaseDSN        string
	bleveDSN           string
	sqliteVecDSN       string
	llmClient          llm.Client
	embeddingsModel    string
	fileConverter      port.FileConverter
	bleveWeight        float64
	sqliteVecWeight    float64
	maxWordsPerSection int
	maxIndexWords      int
	maxTotalWords      int
	taskParallelism    int
	disableHyDE        bool
	disableJudge       bool
	// Low-level options (explicit components)
	index         port.Index
	documentStore port.DocumentStore
	userStore     port.UserStore
	taskRunner    port.TaskRunner
}

func defaultOptions() *options {
	return &options{
		bleveWeight:        0.4,
		sqliteVecWeight:    0.6,
		maxWordsPerSection: 250,
		maxIndexWords:      2000,
		maxTotalWords:      50000,
		taskParallelism:    5,
	}
}

// OptionFunc is a function that configures a Corpus instance.
type OptionFunc func(*options)

// WithStoragePath sets the base directory for all storage files.
// Individual DSNs (database, bleve, sqlitevec) default to subdirectories
// of this path if not explicitly overridden.
func WithStoragePath(dir string) OptionFunc {
	return func(o *options) {
		o.storagePath = dir
	}
}

// WithDatabaseDSN overrides the SQLite database DSN (default: <storagePath>/data.sqlite).
func WithDatabaseDSN(dsn string) OptionFunc {
	return func(o *options) {
		o.databaseDSN = dsn
	}
}

// WithBleveDSN overrides the Bleve index path (default: <storagePath>/index.bleve).
func WithBleveDSN(path string) OptionFunc {
	return func(o *options) {
		o.bleveDSN = path
	}
}

// WithSQLiteVecDSN overrides the SQLiteVec database DSN (default: <storagePath>/index.sqlite).
func WithSQLiteVecDSN(path string) OptionFunc {
	return func(o *options) {
		o.sqliteVecDSN = path
	}
}

// WithLLMClient sets the LLM client used for embeddings, HyDE and Judge transformers.
func WithLLMClient(client llm.Client) OptionFunc {
	return func(o *options) {
		o.llmClient = client
	}
}

// WithEmbeddingsModel sets the embeddings model name used by the SQLiteVec index.
func WithEmbeddingsModel(model string) OptionFunc {
	return func(o *options) {
		o.embeddingsModel = model
	}
}

// WithFileConverter sets a file converter for converting files before indexing.
func WithFileConverter(fc port.FileConverter) OptionFunc {
	return func(o *options) {
		o.fileConverter = fc
	}
}

// WithIndexWeights sets the relative weights for bleve (full-text) and sqlitevec (vector) indexes.
func WithIndexWeights(bleveWeight, sqliteVecWeight float64) OptionFunc {
	return func(o *options) {
		o.bleveWeight = bleveWeight
		o.sqliteVecWeight = sqliteVecWeight
	}
}

// WithMaxWordsPerSection sets the maximum number of words per document section.
func WithMaxWordsPerSection(n int) OptionFunc {
	return func(o *options) {
		o.maxWordsPerSection = n
	}
}

// WithMaxIndexWords sets the maximum number of words indexed per document.
func WithMaxIndexWords(n int) OptionFunc {
	return func(o *options) {
		o.maxIndexWords = n
	}
}

// WithMaxTotalWords sets the maximum total words used by the Judge transformer.
func WithMaxTotalWords(n int) OptionFunc {
	return func(o *options) {
		o.maxTotalWords = n
	}
}

// WithTaskParallelism sets the number of concurrent tasks allowed.
func WithTaskParallelism(n int) OptionFunc {
	return func(o *options) {
		o.taskParallelism = n
	}
}

// WithDisableHyDE disables the HyDE query transformer.
func WithDisableHyDE() OptionFunc {
	return func(o *options) {
		o.disableHyDE = true
	}
}

// WithDisableJudge disables the Judge results transformer.
func WithDisableJudge() OptionFunc {
	return func(o *options) {
		o.disableJudge = true
	}
}

// WithIndex provides a custom port.Index implementation, bypassing auto-composition.
// This allows using custom pipeline configurations (e.g. pgvector, custom weights).
func WithIndex(idx port.Index) OptionFunc {
	return func(o *options) {
		o.index = idx
	}
}

// WithDocumentStore provides a custom port.DocumentStore implementation.
func WithDocumentStore(store port.DocumentStore) OptionFunc {
	return func(o *options) {
		o.documentStore = store
	}
}

// WithUserStore provides a custom port.UserStore implementation.
func WithUserStore(store port.UserStore) OptionFunc {
	return func(o *options) {
		o.userStore = store
	}
}

// WithTaskRunner provides a custom port.TaskRunner implementation.
func WithTaskRunner(runner port.TaskRunner) OptionFunc {
	return func(o *options) {
		o.taskRunner = runner
	}
}

// IndexFileOptions holds options for IndexFile calls.
type IndexFileOptions struct {
	Source      *url.URL
	ETag        string
	Collections []model.CollectionID
}

// IndexFileOptionFunc configures an IndexFile call.
type IndexFileOptionFunc func(*IndexFileOptions)

// WithIndexFileSource sets the source URL for the indexed file.
func WithIndexFileSource(source *url.URL) IndexFileOptionFunc {
	return func(o *IndexFileOptions) {
		o.Source = source
	}
}

// WithIndexFileETag sets the ETag for the indexed file (used for deduplication).
func WithIndexFileETag(etag string) IndexFileOptionFunc {
	return func(o *IndexFileOptions) {
		o.ETag = etag
	}
}

// WithIndexFileCollections associates the indexed file with the given collection IDs.
func WithIndexFileCollections(ids ...model.CollectionID) IndexFileOptionFunc {
	return func(o *IndexFileOptions) {
		o.Collections = ids
	}
}

// SearchOptions holds options for Search calls.
type SearchOptions struct {
	MaxResults  int
	Collections []model.CollectionID
}

// SearchOptionFunc configures a Search call.
type SearchOptionFunc func(*SearchOptions)

// WithSearchMaxResults sets the maximum number of search results.
func WithSearchMaxResults(n int) SearchOptionFunc {
	return func(o *SearchOptions) {
		o.MaxResults = n
	}
}

// WithSearchCollections restricts the search to the given collection IDs.
func WithSearchCollections(ids ...model.CollectionID) SearchOptionFunc {
	return func(o *SearchOptions) {
		o.Collections = ids
	}
}

// AskOptions holds options for Ask calls.
type AskOptions struct {
	SystemPromptTemplate string
}

// AskOptionFunc configures an Ask call.
type AskOptionFunc func(*AskOptions)

// WithAskSystemPromptTemplate overrides the system prompt template.
func WithAskSystemPromptTemplate(tmpl string) AskOptionFunc {
	return func(o *AskOptions) {
		o.SystemPromptTemplate = tmpl
	}
}
