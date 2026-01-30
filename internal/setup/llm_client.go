package setup

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"

	"github.com/bornholm/genai/llm/provider"
	_ "github.com/bornholm/genai/llm/provider/all"
	"github.com/bornholm/genai/llm/retry"

	corpusLLM "github.com/bornholm/corpus/internal/llm"
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

	if conf.LLM.Provider.RateLimit.Enabled {
		slog.DebugContext(ctx, "using rate limited llm client", "rate_limit", conf.LLM.Provider.RateLimit)
		client = corpusLLM.NewPriorizedClient(
			client,
			conf.LLM.Provider.RateLimit.MinInterval, conf.LLM.Provider.RateLimit.MaxBurst, conf.LLM.Provider.RateLimit.LowPriorityThreshold,
		)
	}

	if conf.LLM.Provider.MaxRetries != 0 {
		slog.DebugContext(ctx, "using llm client with retry", "max_retries", conf.LLM.Provider.MaxRetries, "base_backoff", conf.LLM.Provider.BaseBackoff)

		client = retry.Wrap(client, conf.LLM.Provider.BaseBackoff, conf.LLM.Provider.MaxRetries)
	}

	return corpusLLM.NewInstrumentedClient(client, conf.LLM.Provider.ChatCompletionModel, conf.LLM.Provider.EmbeddingsModel), nil
})
