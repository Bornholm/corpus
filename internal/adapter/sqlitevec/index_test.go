package sqlitevec

import (
	"context"
	"os"
	"testing"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/port/testsuite"
	"github.com/bornholm/genai/llm/provider"
	"github.com/bornholm/genai/llm/provider/openai"
	_ "github.com/bornholm/genai/llm/provider/openai"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	tcollama "github.com/testcontainers/testcontainers-go/modules/ollama"
)

func TestIndex(t *testing.T) {
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
	models := []string{chatCompletionModel, embeddingsModel}

	for _, m := range models {
		t.Logf("Pulling model '%s'", m)

		_, _, err = ollamaContainer.Exec(ctx, []string{"ollama", "pull", m})
		if err != nil {
			t.Fatalf("failed to pull model %s: %+v", m, errors.WithStack(err))
		}
	}

	connectionStr, err := ollamaContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get connection string: %+v", errors.WithStack(err))
	}

	client, err := provider.Create(ctx,
		provider.WithChatCompletionOptions(provider.ClientOptions{
			Provider: openai.Name,
			BaseURL:  connectionStr + "/v1/",
			Model:    chatCompletionModel,
		}),
		provider.WithEmbeddingsOptions(provider.ClientOptions{
			Provider: openai.Name,
			BaseURL:  connectionStr + "/v1/",
			Model:    embeddingsModel,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create llm client: %+v", errors.WithStack(err))
	}

	testsuite.TestIndex(t, func(t *testing.T) (port.Index, error) {
		dbFile := "./testdata/test_index.sqlite"

		if err := os.RemoveAll(dbFile); err != nil {
			return nil, errors.WithStack(err)
		}

		db, err := sqlite3.Open(dbFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to open sqlite database")
		}

		index := NewIndex(db, client, 500)

		return index, nil
	})
}
