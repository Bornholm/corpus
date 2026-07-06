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
	Enabled bool `env:"ENABLED,expand" envDefault:"true"`

	RequestInterval             time.Duration `env:"REQUEST_INTERVAL,expand" envDefault:"1s"`
	RequestMaxBurst             int           `env:"REQUEST_MAX_BURST,expand" envDefault:"2"`
	RequestLowPriorityThreshold float64       `env:"REQUEST_LOW_PRIORITY_THRESHOLD,expand" envDefault:"0.5"`

	ChatCompletionTokenMaxBurst int           `env:"CHAT_COMPLETION_TOKEN_MAX_BURST,expand" envDefault:"2000000"`
	ChatCompletionTokenInterval time.Duration `env:"CHAT_COMPLETION_TOKEN_INTERVAL,expand" envDefault:"1m"`

	EmbeddingsTokenMaxBurst int           `env:"EMBEDDINGS_TOKEN_MAX_BURST,expand" envDefault:"20000000"`
	EmbeddingsTokenInterval time.Duration `env:"EMBEDDINGS_TOKEN_INTERVAL,expand" envDefault:"1m"`
}

type LLMIndex struct {
	MaxWords      int `env:"MAX_WORDS,expand" envDefault:"2000"`
	MaxTotalWords int `env:"MAX_TOTAL_WORDS,expand" envDefault:"50000"`

	// GroundingCheck enables the grounding (γ) verifier: after retrieval, an LLM
	// judges whether the evidence supports a reliable answer and the service
	// abstains instead of generating when it does not. Disabled by default.
	GroundingCheck    bool    `env:"GROUNDING_CHECK,expand" envDefault:"false"`
	GroundingMinScore float64 `env:"GROUNDING_MIN_SCORE,expand" envDefault:"0.4"`

	// IterativeRetrieval enables grounding-driven re-retrieval: when the evidence
	// is not confidently grounded, the query is reformulated and searched again
	// (up to IterativeMaxRounds) before answering. Requires GroundingCheck.
	IterativeRetrieval bool `env:"ITERATIVE_RETRIEVAL,expand" envDefault:"false"`
	IterativeMaxRounds int  `env:"ITERATIVE_MAX_ROUNDS,expand" envDefault:"1"`

	// QueryDecomposition enables splitting a complex question into sub-questions,
	// searching each and fusing their evidence before answering.
	QueryDecomposition         bool `env:"QUERY_DECOMPOSITION,expand" envDefault:"false"`
	DecompositionMaxSubQueries int  `env:"DECOMPOSITION_MAX_SUBQUERIES,expand" envDefault:"3"`
}
