package setup

import (
	"context"
	"log/slog"
	"time"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"

	"github.com/bornholm/genai/llm/provider"
	_ "github.com/bornholm/genai/llm/provider/openai"
)

var getLLMClientFromConfig = createFromConfigOnce(func(ctx context.Context, conf *config.Config) (llm.Client, error) {
	client, err := provider.Create(ctx, provider.WithConfig(&provider.Config{
		Provider:            provider.Name(conf.LLM.Provider.Name),
		BaseURL:             conf.LLM.Provider.BaseURL,
		Key:                 conf.LLM.Provider.Key,
		ChatCompletionModel: conf.LLM.Provider.ChatCompletionModel,
		EmbeddingsModel:     conf.LLM.Provider.EmbeddingsModel,
	}))

	if err != nil {
		return nil, errors.WithStack(err)
	}

	if conf.LLM.Provider.RateLimit != 0 {
		return NewRateLimitedClient(client, conf.LLM.Provider.RateLimit), nil
	}

	return client, nil
})

type RateLimitedClient struct {
	limiter *rate.Limiter
	client  llm.Client
}

// ChatCompletion implements llm.Client.
func (r *RateLimitedClient) ChatCompletion(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (llm.CompletionResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, errors.WithStack(err)
	}
	slog.DebugContext(ctx, "retrieving chat completing")
	return r.client.ChatCompletion(ctx, funcs...)
}

// Embeddings implements llm.Client.
func (r *RateLimitedClient) Embeddings(ctx context.Context, funcs ...llm.EmbeddingsOptionFunc) (llm.EmbeddingsResponse, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, errors.WithStack(err)
	}
	slog.DebugContext(ctx, "retrieving embeddings")
	return r.client.Embeddings(ctx, funcs...)
}

func NewRateLimitedClient(client llm.Client, minDelay time.Duration) *RateLimitedClient {
	return &RateLimitedClient{
		limiter: rate.NewLimiter(rate.Every(minDelay), 1),
		client:  client,
	}
}

var _ llm.Client = &RateLimitedClient{}
