package llm

import (
	"context"

	"github.com/bornholm/corpus/internal/metrics"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type InstrumentedClient struct {
	client              llm.Client
	chatCompletionModel string
	embeddingsModel     string
}

// ChatCompletionStream implements [llm.Client].
func (c *InstrumentedClient) ChatCompletionStream(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (<-chan llm.StreamChunk, error) {
	sourceChan, err := c.client.ChatCompletionStream(ctx, funcs...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Create a new channel to wrap the source channel and add metrics
	wrappedChan := make(chan llm.StreamChunk)

	go func() {
		defer close(wrappedChan)

		var totalUsage llm.ChatCompletionUsage

		for chunk := range sourceChan {
			wrappedChan <- chunk

			if usage := chunk.Usage(); usage != nil {
				totalUsage = usage
			}
		}

		if totalUsage != nil {
			metrics.CompletionTokens.With(prometheus.Labels{
				metrics.LabelModel: c.chatCompletionModel,
				metrics.LabelType:  "chat_completion",
			}).Add(float64(totalUsage.CompletionTokens()))

			metrics.TotalTokens.With(prometheus.Labels{
				metrics.LabelModel: c.chatCompletionModel,
				metrics.LabelType:  "chat_completion",
			}).Add(float64(totalUsage.TotalTokens()))

			metrics.PromptTokens.With(prometheus.Labels{
				metrics.LabelModel: c.chatCompletionModel,
				metrics.LabelType:  "chat_completion",
			}).Add(float64(totalUsage.PromptTokens()))
		}
	}()

	return wrappedChan, nil
}

// ChatCompletion implements llm.Client.
func (c *InstrumentedClient) ChatCompletion(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (llm.ChatCompletionResponse, error) {
	res, err := c.client.ChatCompletion(ctx, funcs...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if usage := res.Usage(); usage != nil {
		metrics.CompletionTokens.With(prometheus.Labels{
			metrics.LabelModel: c.chatCompletionModel,
			metrics.LabelType:  "chat_completion",
		}).Add(float64(usage.CompletionTokens()))

		metrics.TotalTokens.With(prometheus.Labels{
			metrics.LabelModel: c.chatCompletionModel,
			metrics.LabelType:  "chat_completion",
		}).Add(float64(usage.TotalTokens()))

		metrics.PromptTokens.With(prometheus.Labels{
			metrics.LabelModel: c.chatCompletionModel,
			metrics.LabelType:  "chat_completion",
		}).Add(float64(usage.PromptTokens()))
	}

	return res, nil
}

// Embeddings implements llm.Client.
func (c *InstrumentedClient) Embeddings(ctx context.Context, input string, funcs ...llm.EmbeddingsOptionFunc) (llm.EmbeddingsResponse, error) {
	res, err := c.client.Embeddings(ctx, input, funcs...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if usage := res.Usage(); usage != nil {
		metrics.TotalTokens.With(prometheus.Labels{
			metrics.LabelModel: c.embeddingsModel,
			metrics.LabelType:  "embeddings",
		}).Add(float64(usage.TotalTokens()))

		metrics.PromptTokens.With(prometheus.Labels{
			metrics.LabelModel: c.embeddingsModel,
			metrics.LabelType:  "embeddings",
		}).Add(float64(usage.PromptTokens()))
	}

	return res, nil
}

func NewInstrumentedClient(client llm.Client, chatCompletionModel string, embeddingsModel string) *InstrumentedClient {
	return &InstrumentedClient{
		client:              client,
		chatCompletionModel: chatCompletionModel,
		embeddingsModel:     embeddingsModel,
	}
}

var _ llm.Client = &InstrumentedClient{}
