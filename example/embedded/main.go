// Package main demonstrates two ways to use the corpus embedded library.
//
// LLM configuration is read from environment variables via the genai provider/env package.
//
// Usage:
//
//	go run ./example/embedded/main.go [--advanced] <storage-dir>
//
// Examples:
//
//	# Simple (auto-composition)
//	LLM_CHAT_COMPLETION_PROVIDER=openai \
//	LLM_CHAT_COMPLETION_OPENAI_BASE_URL=http://localhost:11434/v1 \
//	LLM_EMBEDDINGS_PROVIDER=openai \
//	LLM_EMBEDDINGS_OPENAI_BASE_URL=http://localhost:11434/v1 \
//	go run ./example/embedded/main.go /tmp/corpus-demo
//
//	# Advanced (explicit pipeline), loading env from a .env file
//	go run ./example/embedded/main.go --advanced /tmp/corpus-demo
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	bleve "github.com/blevesearch/bleve/v2"
	bleveAdapter "github.com/bornholm/corpus/pkg/adapter/bleve"
	gormAdapter "github.com/bornholm/corpus/pkg/adapter/gorm"
	"github.com/bornholm/corpus/pkg/adapter/pipeline"
	sqlitevecAdapter "github.com/bornholm/corpus/pkg/adapter/sqlitevec"
	"github.com/bornholm/corpus/pkg/corpus"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/genai/llm"
	"github.com/bornholm/genai/llm/provider"
	providerenv "github.com/bornholm/genai/llm/provider/env"
	"github.com/ncruces/go-sqlite3"
	gormlite "github.com/ncruces/go-sqlite3/gormlite"
	gormdb "gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	// Register sqlite-vec extension
	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

var advanced bool

func init() {
	flag.BoolVar(&advanced, "advanced", false, "use advanced options")
}

func main() {
	flag.Parse()

	storageDir := flag.Arg(0)

	if storageDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s [-advanced] <storage-dir>\n", os.Args[0])
		os.Exit(1)
	}

	ctx := context.Background()

	llmClient, err := newLLMClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "LLM client error: %+v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(storageDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir error: %+v\n", err)
		os.Exit(1)
	}

	var c *corpus.Corpus
	if advanced {
		c, err = advancedSetup(ctx, storageDir, llmClient)
	} else {
		c, err = simpleSetup(ctx, storageDir, llmClient)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup error: %+v\n", err)
		os.Exit(1)
	}

	// Create a collection
	collID, err := c.CreateCollection(ctx, "demo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "CreateCollection error: %+v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Collection created: %s\n", collID)

	// Index a small in-memory document
	content := strings.NewReader(`# Go Programming Language

Go is a statically typed, compiled programming language designed at Google.
It is syntactically similar to C, but with memory safety, garbage collection,
structural typing, and CSP-style concurrency.

## Concurrency with Goroutines and Channels

Go's concurrency model is built around goroutines and channels, inspired by
Communicating Sequential Processes (CSP).

A goroutine is a lightweight thread managed by the Go runtime. You can start
thousands of goroutines simultaneously. To launch a goroutine, use the "go"
keyword before a function call:

    go doSomething()

Channels are the pipes that connect concurrent goroutines. You can send values
into channels from one goroutine and receive those values into another goroutine.
This design avoids shared memory and makes concurrent code safer and easier to reason about.

## Key Features

- Fast compilation
- Garbage collection
- Strong static typing
- Built-in concurrency with goroutines and channels
- Simple, clean syntax
`)

	sourceURL, _ := url.Parse("example://demo/go-intro.md")

	taskID, err := c.IndexFile(ctx, collID, "go-intro.md", content,
		corpus.WithIndexFileSource(sourceURL),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "IndexFile error: %+v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Indexing task scheduled: %s\n", taskID)

	// Wait for the task to complete
	fmt.Println("Waiting for indexing to complete...")
	for {
		state, err := c.GetTaskState(ctx, taskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GetTaskState error: %+v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  Task status: %s\n", state.Status)
		if state.Status == port.TaskStatusSucceeded || state.Status == port.TaskStatusFailed {
			if state.Error != nil {
				fmt.Fprintf(os.Stderr, "Indexing failed: %v\n", state.Error)
				os.Exit(1)
			}
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Search
	fmt.Println("\nSearching for 'concurrency goroutines'...")
	results, err := c.Search(ctx, "concurrency goroutines",
		corpus.WithSearchCollections(collID),
		corpus.WithSearchMaxResults(3),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search error: %+v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d result(s):\n", len(results))
	for i, r := range results {
		fmt.Printf("  [%d] Source: %s — %d section(s)\n", i+1, r.Source, len(r.Sections))
	}

	if len(results) == 0 {
		fmt.Println("No results (LLM may not be available, or indexing needs more time)")
		return
	}

	// Ask
	fmt.Println("\nAsking: 'What are the key features of Go?'")
	answer, _, err := c.Ask(ctx, "What are the key features of Go?", results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ask error: %+v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nAnswer:\n%s\n", answer)
}

// simpleSetup demonstrates the high-level API with auto-composition.
// This is the easiest way to get started: just provide a storage directory
// and an LLM client.
func simpleSetup(ctx context.Context, storageDir string, llmClient llm.Client) (*corpus.Corpus, error) {
	fmt.Println("=== Simple setup (auto-composition) ===")

	return corpus.New(ctx,
		corpus.WithStoragePath(storageDir),
		corpus.WithLLMClient(llmClient),
		// Optionally disable expensive LLM transformers:
		// corpus.WithDisableHyDE(),
		// corpus.WithDisableJudge(),
	)
}

// advancedSetup demonstrates the low-level API with explicit component wiring.
// This approach allows full control over each component, making it straightforward
// to swap implementations (e.g. replacing SQLite with PostgreSQL + pgvector).
func advancedSetup(ctx context.Context, storageDir string, llmClient llm.Client) (*corpus.Corpus, error) {
	fmt.Println("=== Advanced setup (explicit pipeline) ===")

	// 1. Open the GORM SQLite database.
	db, err := gormdb.Open(gormlite.Open(storageDir+"/data.sqlite"), &gormdb.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Error),
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm db: %w", err)
	}
	internalDB, _ := db.DB()
	internalDB.SetMaxOpenConns(1)
	db.Exec("PRAGMA journal_mode=wal; PRAGMA foreign_keys=on; PRAGMA busy_timeout=5000")

	store := gormAdapter.NewStore(db)

	// 2. Open the Bleve full-text index.
	blevePath := storageDir + "/index.bleve"
	var rawBleveIdx bleve.Index
	if _, err := os.Stat(blevePath); os.IsNotExist(err) {
		rawBleveIdx, err = bleve.New(blevePath, bleveAdapter.IndexMapping())
		if err != nil {
			return nil, fmt.Errorf("create bleve index: %w", err)
		}
	} else {
		rawBleveIdx, err = bleve.Open(blevePath)
		if err != nil {
			return nil, fmt.Errorf("open bleve index: %w", err)
		}
	}
	bleveIdx := bleveAdapter.NewIndex(rawBleveIdx)

	// 3. Open the SQLiteVec vector index.
	sqliteConn, err := sqlite3.Open(storageDir + "/index.sqlite")
	if err != nil {
		return nil, fmt.Errorf("open sqlitevec: %w", err)
	}
	vecIdx := sqlitevecAdapter.NewIndex(sqliteConn, llmClient, "", 2000)

	// 4. Assemble a custom pipeline (40% full-text, 60% vector).
	pipelineIdx := pipeline.NewIndex(
		pipeline.WeightedIndexes{
			pipeline.NewIdentifiedIndex("bleve", bleveIdx): 0.4,
			pipeline.NewIdentifiedIndex("vec", vecIdx):     0.6,
		},
		pipeline.WithQueryTransformers(
			pipeline.NewHyDEQueryTransformer(llmClient, store),
		),
		pipeline.WithResultsTransformers(
			pipeline.NewDuplicateContentResultsTransformer(store),
			pipeline.NewJudgeResultsTransformer(llmClient, store, 50000),
		),
	)

	// 5. Wire everything together via the low-level OptionFuncs.
	//    Swapping to PostgreSQL + pgvector would only require changing steps 1-4
	//    while keeping this wiring identical.
	return corpus.New(ctx,
		corpus.WithIndex(pipelineIdx),
		corpus.WithDocumentStore(store),
		corpus.WithUserStore(store),
		corpus.WithLLMClient(llmClient),
	)
}

func newLLMClient(ctx context.Context) (llm.Client, error) {
	return provider.Create(ctx,
		providerenv.With("LLM_", ".env"),
	)
}
