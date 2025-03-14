package config

import "time"

type LLM struct {
	Provider LLMProvider `envPrefix:"PROVIDER_"`
}

type LLMProvider struct {
	Name                string        `env:"NAME" envDefault:"openai"`
	BaseURL             string        `env:"BASE_URL" envDefault:"https://api.openai.com/v1/"`
	Key                 string        `env:"KEY"`
	ChatCompletionModel string        `env:"CHAT_COMPLETION_MODEL" envDefault:"gpt-4o-mini"`
	EmbeddingsModel     string        `env:"EMBEDDINGS_MODEL" envDefault:"text-embedding-3-large"`
	RateLimit           time.Duration `env:"RATE_LIMIT" envDefault:"1500ms"`
}
