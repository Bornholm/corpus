package llm

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/genai/llm"
	"github.com/bornholm/go-x/slogx"
)

type LoggerClient struct {
	client llm.Client
}

// ChatCompletion implements llm.Client.
func (c *LoggerClient) ChatCompletion(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (llm.ChatCompletionResponse, error) {
	ctx = slogx.WithAttrs(ctx, slog.String("llm_request", "chat_completion"))

	before := time.Now()
	defer func() {
		slog.DebugContext(ctx, "llm request completed", slog.Duration("duration", time.Since(before)))
	}()

	slog.DebugContext(ctx, "llm request started")

	return c.client.ChatCompletion(ctx, funcs...)
}

// ChatCompletionStream implements llm.Client.
func (c *LoggerClient) ChatCompletionStream(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (<-chan llm.StreamChunk, error) {
	ctx = slogx.WithAttrs(ctx, slog.String("llm_request", "chat_completion_stream"))

	before := time.Now()
	defer func() {
		slog.DebugContext(ctx, "llm request completed", slog.Duration("duration", time.Since(before)))
	}()

	slog.DebugContext(ctx, "llm request started")

	return c.client.ChatCompletionStream(ctx, funcs...)
}

// Embeddings implements llm.Client.
func (c *LoggerClient) Embeddings(ctx context.Context, inputs []string, funcs ...llm.EmbeddingsOptionFunc) (llm.EmbeddingsResponse, error) {
	ctx = slogx.WithAttrs(ctx, slog.String("llm_request", "embeddings"))

	before := time.Now()
	defer func() {
		slog.DebugContext(ctx, "llm request completed", slog.Duration("duration", time.Since(before)))
	}()

	slog.DebugContext(ctx, "llm request started")

	return c.client.Embeddings(ctx, inputs, funcs...)
}

func NewLoggerClient(client llm.Client) *LoggerClient {
	return &LoggerClient{
		client: client,
	}
}

var _ llm.Client = &LoggerClient{}
