package llm

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/genai/llm"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

type contextKey int

const contextKeyHighPriority contextKey = iota

func WithHighPriority(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKeyHighPriority, true)
}

func WithoutHighPriority(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKeyHighPriority, false)
}

func isHighPriority(ctx context.Context) bool {
	highPriority, ok := ctx.Value(contextKeyHighPriority).(bool)
	if !ok {
		return false
	}

	return highPriority
}

type PriorizedClient struct {
	limiter *PriorityLimiter
	client  llm.Client
}

// ChatCompletion implements llm.Client.
func (c *PriorizedClient) ChatCompletion(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (llm.ChatCompletionResponse, error) {
	ctx, err := c.wait(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return c.client.ChatCompletion(ctx, funcs...)
}

// ChatCompletionStream implements llm.Client.
func (c *PriorizedClient) ChatCompletionStream(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (<-chan llm.StreamChunk, error) {
	ctx, err := c.wait(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return c.client.ChatCompletionStream(ctx, funcs...)
}

// Embeddings implements llm.Client.
func (c *PriorizedClient) Embeddings(ctx context.Context, inputs []string, funcs ...llm.EmbeddingsOptionFunc) (llm.EmbeddingsResponse, error) {
	ctx, err := c.wait(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return c.client.Embeddings(ctx, inputs, funcs...)
}

func (c *PriorizedClient) wait(ctx context.Context) (context.Context, error) {
	isHighPriority := isHighPriority(ctx)

	ctx = slogx.WithAttrs(ctx, slog.Bool("high_priority", isHighPriority))

	slog.DebugContext(ctx, "waiting for next available slot")

	if err := c.limiter.Wait(ctx, 1, isHighPriority); err != nil {
		return ctx, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "slot acquired")

	return ctx, nil
}

func NewPriorizedClient(client llm.Client, minInterval time.Duration, maxBurst int, lowPriorityThreshold float64) *PriorizedClient {
	return &PriorizedClient{
		limiter: NewPriorityLimiter(rate.Every(minInterval), maxBurst, lowPriorityThreshold),
		client:  client,
	}
}

var _ llm.Client = &PriorizedClient{}
