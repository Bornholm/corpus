package llm

import (
	"context"
	"time"

	"github.com/bornholm/genai/llm"
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
	chatCompletionLimiter *PriorityLimiter
	embeddingsLimiter     *PriorityLimiter
	client                llm.Client
}

// ChatCompletion implements llm.Client.
func (c *PriorizedClient) ChatCompletion(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (llm.ChatCompletionResponse, error) {
	if !c.allow(ctx, c.chatCompletionLimiter) {
		return nil, errors.WithStack(llm.ErrRateLimit)
	}
	return c.client.ChatCompletion(ctx, funcs...)
}

// ChatCompletionStream implements llm.Client.
func (c *PriorizedClient) ChatCompletionStream(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (<-chan llm.StreamChunk, error) {
	if !c.allow(ctx, c.chatCompletionLimiter) {
		return nil, errors.WithStack(llm.ErrRateLimit)
	}
	return c.client.ChatCompletionStream(ctx, funcs...)
}

// Embeddings implements llm.Client.
func (c *PriorizedClient) Embeddings(ctx context.Context, input string, funcs ...llm.EmbeddingsOptionFunc) (llm.EmbeddingsResponse, error) {
	if !c.allow(ctx, c.embeddingsLimiter) {
		return nil, errors.WithStack(llm.ErrRateLimit)
	}
	return c.client.Embeddings(ctx, input, funcs...)
}

func (c *PriorizedClient) allow(ctx context.Context, limiter *PriorityLimiter) bool {
	isHighPriority := isHighPriority(ctx)
	return limiter.Allow(isHighPriority)
}

func NewPriorizedClient(client llm.Client, chatCompletionMinInterval time.Duration, chatCompletionMaxBurst int, chatCompletionThreshold float64, embeddingsMinInterval time.Duration, embeddingsMaxBurst int, embeddingsThreshold float64) *PriorizedClient {
	return &PriorizedClient{
		chatCompletionLimiter: NewPriorityLimiter(rate.Every(chatCompletionMinInterval), chatCompletionMaxBurst, chatCompletionThreshold),
		embeddingsLimiter:     NewPriorityLimiter(rate.Every(embeddingsMinInterval), embeddingsMaxBurst, embeddingsThreshold),
		client:                client,
	}
}

var _ llm.Client = &PriorizedClient{}
