package config

import "time"

type FileConverter struct {
	Enabled   bool                   `env:"ENABLED,expand" envDefault:"true"`
	URI       []string               `env:"URI,expand" envDefault:"libreoffice+pandoc://" envSeparator:","`
	RateLimit FileConverterRateLimit `envPrefix:"RATE_LIMIT_"`

	MaxRetries  int           `env:"MAX_RETRIES,expand" envDefault:"3"`
	BaseBackoff time.Duration `env:"BASE_BACKOFF" envDefault:"2s"`
}

type FileConverterRateLimit struct {
	Enabled bool `env:"ENABLED,expand" envDefault:"true"`

	RequestInterval time.Duration `env:"REQUEST_INTERVAL,expand" envDefault:"1s"`
	RequestMaxBurst int           `env:"REQUEST_MAX_BURST,expand" envDefault:"1"`
}
