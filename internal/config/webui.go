package config

type WebUI struct {
	Enabled bool `env:"ENABLED,expand" envDefault:"true"`
}
