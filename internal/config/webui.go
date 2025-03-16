package config

type WebUI struct {
	Enabled bool `env:"ENABLED" envDefault:"true"`
}
