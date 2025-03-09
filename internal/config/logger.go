package config

type Logger struct {
	Level int `env:"LEVEL" envDefault:"0"`
}
