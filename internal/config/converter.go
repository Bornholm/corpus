package config

type FileConverter struct {
	Enabled bool     `env:"ENABLED,expand" envDefault:"true"`
	URI     []string `env:"URI,expand" envDefault:"pandoc://" envSeparator:","`
}
