package setup

import (
	"context"
	"time"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/metrics"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	"github.com/bornholm/genai/llm/provider"
	_ "github.com/bornholm/genai/llm/provider/openai"
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
		client = NewRateLimitedClient(client, conf.LLM.Provider.RateLimit)
	}

	return NewInstrumentedClient(client, conf.LLM.Provider.ChatCompletionModel, conf.LLM.Provider.EmbeddingsModel), nil
})

type InstrumentedClient struct {
	client              llm.Client
	chatCompletionModel string
	embeddingsModel     string
}

// ExtractText implements llm.Client.
func (c *InstrumentedClient) ExtractText(ctx context.Context, funcs ...llm.ExtractTextOptionFunc) (llm.ExtractTextResponse, error) {
	res, err := c.client.ExtractText(ctx, funcs...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return res, nil
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

type RateLimitedClient struct {
	limiter *rate.Limiter
	client  llm.Client
}

// ExtractText implements llm.Client.
func (r *RateLimitedClient) ExtractText(ctx context.Context, funcs ...llm.ExtractTextOptionFunc) (llm.ExtractTextResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, errors.WithStack(err)
	}
	return r.client.ExtractText(ctx, funcs...)
}

// ChatCompletion implements llm.Client.
func (r *RateLimitedClient) ChatCompletion(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (llm.ChatCompletionResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, errors.WithStack(err)
	}
	return r.client.ChatCompletion(ctx, funcs...)
}

// Embeddings implements llm.Client.
func (r *RateLimitedClient) Embeddings(ctx context.Context, input string, funcs ...llm.EmbeddingsOptionFunc) (llm.EmbeddingsResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, errors.WithStack(err)
	}
	return r.client.Embeddings(ctx, input, funcs...)
}

func NewRateLimitedClient(client llm.Client, minDelay time.Duration) *RateLimitedClient {
	return &RateLimitedClient{
		limiter: rate.NewLimiter(rate.Every(minDelay), 1),
		client:  client,
	}
}

var _ llm.Client = &RateLimitedClient{}
