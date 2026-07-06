package corpus_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bornholm/corpus/pkg/corpus"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/genai/llm/provider"
	"github.com/bornholm/genai/llm/provider/openai"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	tcollama "github.com/testcontainers/testcontainers-go/modules/ollama"
)

// TestEndToEndRAG exercises the full embedded pipeline against a real
// LLM/embeddings backend (ollama in a container), with the three MothRAG-derived
// mechanisms enabled: grounding (γ) check + abstention, iterative re-retrieval
// and query decomposition. It mirrors pkg/adapter/sqlitevec/index_test.go for
// the container setup.
//
// Skipped with `go test -short` (pulls models, needs docker).
func TestEndToEndRAG(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: requires docker and model pulls")
	}

	ctx := context.Background()

	t.Logf("Starting ollama container")

	ollamaContainer, err := tcollama.Run(ctx, "ollama/ollama:0.5.7", testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Mounts: testcontainers.ContainerMounts{
				{
					Source: testcontainers.GenericVolumeMountSource{
						Name: "ollama-data",
					},
					Target: "/root/.ollama",
				},
			},
		},
	}))
	defer func() {
		if err := testcontainers.TerminateContainer(ollamaContainer); err != nil {
			t.Fatalf("failed to terminate container: %+v", errors.WithStack(err))
		}
	}()
	if err != nil {
		t.Fatalf("failed to start container: %+v", err)
	}

	chatCompletionModel := "qwen2.5:3b"
	embeddingsModel := "mxbai-embed-large:latest"

	for _, m := range []string{chatCompletionModel, embeddingsModel} {
		t.Logf("Pulling model '%s'", m)
		if _, _, err := ollamaContainer.Exec(ctx, []string{"ollama", "pull", m}); err != nil {
			t.Fatalf("failed to pull model %s: %+v", m, errors.WithStack(err))
		}
	}

	connectionStr, err := ollamaContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get connection string: %+v", errors.WithStack(err))
	}

	client, err := provider.Create(ctx,
		provider.WithChatCompletion(openai.Name, openai.Options{
			CommonOptions: provider.CommonOptions{
				BaseURL: connectionStr + "/v1/",
				Model:   chatCompletionModel,
			},
		}),
		provider.WithEmbeddings(openai.Name, openai.Options{
			CommonOptions: provider.CommonOptions{
				BaseURL: connectionStr + "/v1/",
				Model:   embeddingsModel,
			},
		}),
	)
	if err != nil {
		t.Fatalf("failed to create llm client: %+v", errors.WithStack(err))
	}

	// Auto-composed corpus with all three MothRAG features enabled.
	c, err := corpus.New(ctx,
		corpus.WithStoragePath(t.TempDir()),
		corpus.WithLLMClient(client),
		corpus.WithEmbeddingsModel(embeddingsModel),
		corpus.WithGroundingCheck(),
		corpus.WithGroundingMinScore(0.4),
		corpus.WithIterativeRetrieval(1),
		corpus.WithQueryDecomposition(3),
	)
	if err != nil {
		t.Fatalf("could not create corpus: %+v", errors.WithStack(err))
	}

	collID, err := c.CreateCollection(ctx, "e2e")
	if err != nil {
		t.Fatalf("could not create collection: %+v", errors.WithStack(err))
	}

	// Two documents forming a 2-hop chain: the answer to "which country is the
	// Eiffel Tower in" requires linking (Eiffel Tower → Paris) with (Paris →
	// France) across the two documents.
	indexAndWait(ctx, t, c, collID, "eiffel.md", "example://e2e/eiffel.md",
		"# The Eiffel Tower\n\nThe Eiffel Tower is a wrought-iron lattice tower located in Paris. "+
			"It was completed in 1889 and is one of the most recognisable structures in Paris.")
	indexAndWait(ctx, t, c, collID, "paris.md", "example://e2e/paris.md",
		"# Paris\n\nParis is the capital and most populous city of France. "+
			"It sits on the river Seine in the north of France.")

	t.Run("grounded multi-hop question", func(t *testing.T) {
		result, err := c.AskWithRetrieval(ctx, "In which country is the Eiffel Tower located?",
			corpus.WithSearchCollections(collID),
		)
		if err != nil {
			t.Fatalf("AskWithRetrieval error: %+v", errors.WithStack(err))
		}

		t.Logf("answer=%q rounds=%d grounding=%+v results=%d",
			result.Answer, result.Rounds, result.Grounding, len(result.Results))

		if len(result.Results) == 0 {
			t.Fatalf("expected retrieved evidence for a supported question, got none")
		}
		if result.Grounding == nil {
			t.Fatalf("expected a grounding verdict (checker is enabled)")
		}
		if strings.TrimSpace(result.Answer) == "" {
			t.Fatalf("expected a non-empty answer")
		}

		// Qualitative (soft): a correct grounded answer should mention France and
		// should not be the abstention message.
		if isAbstention(result.Answer) {
			t.Logf("WARNING: abstained on a supported question (small model): %q", result.Answer)
		} else if !strings.Contains(strings.ToLower(result.Answer), "france") {
			t.Logf("WARNING: answer does not mention 'France': %q", result.Answer)
		}
	})

	t.Run("abstains / stays safe on unsupported question", func(t *testing.T) {
		result, err := c.AskWithRetrieval(ctx, "Who won the 2018 FIFA World Cup final?",
			corpus.WithSearchCollections(collID),
		)
		if err != nil {
			t.Fatalf("AskWithRetrieval error: %+v", errors.WithStack(err))
		}

		t.Logf("answer=%q rounds=%d grounding=%+v results=%d",
			result.Answer, result.Rounds, result.Grounding, len(result.Results))

		// Safe behaviour: either nothing was retrieved, or the grounding gate led
		// to an abstention rather than a confident (necessarily hallucinated)
		// answer. A tiny model can misjudge, so a confident answer is only logged.
		safe := len(result.Results) == 0 || isAbstention(result.Answer)
		if !safe {
			t.Logf("WARNING: produced a non-abstention answer with no supporting evidence: %q", result.Answer)
		}
		if result.Grounding == nil && len(result.Results) > 0 {
			t.Fatalf("grounding checker should have run when evidence was retrieved")
		}
	})
}

// isAbstention reports whether an answer is the grounding-gate abstention message
// (its stable prefix, mirroring service's defaultAbstentionMessage).
func isAbstention(answer string) bool {
	return strings.HasPrefix(answer, "I cannot provide a reliable answer")
}

// indexAndWait indexes an in-memory document and blocks until the indexing task
// completes, failing the test on error or timeout.
func indexAndWait(ctx context.Context, t *testing.T, c *corpus.Corpus, collID model.CollectionID, filename, source, content string) {
	t.Helper()

	sourceURL, err := url.Parse(source)
	if err != nil {
		t.Fatalf("invalid source url %q: %+v", source, errors.WithStack(err))
	}

	taskID, err := c.IndexFile(ctx, collID, filename, strings.NewReader(content),
		corpus.WithIndexFileSource(sourceURL),
	)
	if err != nil {
		t.Fatalf("IndexFile(%s) error: %+v", filename, errors.WithStack(err))
	}

	deadline := time.Now().Add(2 * time.Minute)
	for {
		state, err := c.GetTaskState(ctx, taskID)
		if err != nil {
			t.Fatalf("GetTaskState error: %+v", errors.WithStack(err))
		}

		switch state.Status {
		case port.TaskStatusSucceeded:
			return
		case port.TaskStatusFailed:
			t.Fatalf("indexing of %s failed: %v", filename, state.Error)
		}

		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for indexing of %s (last status: %s)", filename, state.Status)
		}

		time.Sleep(200 * time.Millisecond)
	}
}
