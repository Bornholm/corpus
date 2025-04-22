package config

type FileConverter struct {
	Enabled bool     `env:"ENABLED" envDefault:"true"`
	URI     []string `env:"URI" envDefault:"pandoc://" envSeparator:","`
}
