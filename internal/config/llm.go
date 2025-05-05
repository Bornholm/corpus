package config

import "time"

type LLM struct {
	Provider LLMProvider `envPrefix:"PROVIDER_"`
	Index    LLMIndex    `envPrefix:"INDEX_"`
}

type LLMProvider struct {
	Name                string        `env:"NAME,expand" envDefault:"openai"`
	BaseURL             string        `env:"BASE_URL,expand" envDefault:"https://api.openai.com/v1/"`
	Key                 string        `env:"KEY,expand"`
	ChatCompletionModel string        `env:"CHAT_COMPLETION_MODEL,expand" envDefault:"gpt-4o-mini"`
	EmbeddingsModel     string        `env:"EMBEDDINGS_MODEL,expand" envDefault:"text-embedding-3-large"`
	RateLimit           time.Duration `env:"RATE_LIMIT,expand" envDefault:"1500ms"`
}

type LLMIndex struct {
	MaxWords int `env:"MAX_WORDS,expand" envDefault:"2000"`
}
