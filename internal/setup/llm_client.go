package setup

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/metrics"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/bornholm/genai/llm/provider"
	_ "github.com/bornholm/genai/llm/provider/all"
	"github.com/bornholm/genai/llm/ratelimit"
	"github.com/bornholm/genai/llm/retry"
)

var getLLMClientFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (llm.Client, error) {
	client, err := provider.Create(ctx,
		provider.WithChatCompletionOptions(provider.ClientOptions{
			Provider: provider.Name(conf.LLM.Provider.Name),
			BaseURL:  conf.LLM.Provider.BaseURL,
			APIKey:   conf.LLM.Provider.Key,
			Model:    conf.LLM.Provider.ChatCompletionModel,
		}),
		provider.WithEmbeddingsOptions(provider.ClientOptions{
			Provider: provider.Name(conf.LLM.Provider.Name),
			BaseURL:  conf.LLM.Provider.BaseURL,
			APIKey:   conf.LLM.Provider.Key,
			Model:    conf.LLM.Provider.EmbeddingsModel,
		}),
	)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	if conf.LLM.Provider.RateLimit != 0 {
		slog.DebugContext(ctx, "using rate limited llm client", "rate", conf.LLM.Provider.RateLimit)
		client = ratelimit.Wrap(client, conf.LLM.Provider.RateLimit, 1)
	}

	if conf.LLM.Provider.MaxRetries != 0 {
		slog.DebugContext(ctx, "using llm client with retry", "max_retries", conf.LLM.Provider.MaxRetries, "base_backoff", conf.LLM.Provider.BaseBackoff)

		client = retry.Wrap(client, conf.LLM.Provider.BaseBackoff, conf.LLM.Provider.MaxRetries)
	}

	return NewInstrumentedClient(client, conf.LLM.Provider.ChatCompletionModel, conf.LLM.Provider.EmbeddingsModel), nil
})

type InstrumentedClient struct {
	client              llm.Client
	chatCompletionModel string
	embeddingsModel     string
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
