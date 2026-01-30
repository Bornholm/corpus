package config

import "time"

type LLM struct {
	Provider LLMProvider `envPrefix:"PROVIDER_"`
	Index    LLMIndex    `envPrefix:"INDEX_"`
}

type LLMProvider struct {
	Name                string `env:"NAME,expand" envDefault:"openai"`
	BaseURL             string `env:"BASE_URL,expand" envDefault:"https://api.mistral.ai/v1/"`
	Key                 string `env:"KEY,expand"`
	ChatCompletionModel string `env:"CHAT_COMPLETION_MODEL,expand" envDefault:"mistral-small-latest"`
	EmbeddingsModel     string `env:"EMBEDDINGS_MODEL,expand" envDefault:"mistral-embed"`

	RateLimit   LLMRateLimit  `envPrefix:"RATE_LIMIT_"`
	MaxRetries  int           `env:"MAX_RETRIES,expand" envDefault:"3"`
	BaseBackoff time.Duration `env:"BASE_BACKOFF" envDefault:"1s"`
}

type LLMRateLimit struct {
	Enabled              bool          `env:"ENABLED,expand" envDefault:"true"`
	MinInterval          time.Duration `env:"MIN_INTERVAL,expand" envDefault:"1s"`
	MaxBurst             int           `env:"MAX_BURST,expand" envDefault:"2"`
	LowPriorityThreshold float64       `env:"LOW_PRIORITY_THRESHOLD,expand" envDefault:"0.5"`
}

type LLMIndex struct {
	MaxWords int `env:"MAX_WORDS,expand" envDefault:"2000"`
}
