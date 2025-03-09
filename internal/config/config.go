package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/pkg/errors"
)

type Config struct {
	Logger  Logger  `envPrefix:"LOGGER_"`
	HTTP    HTTP    `envPrefix:"HTTP_"`
	Storage Storage `envPrefix:"STORAGE_"`
	LLM     LLM     `envPrefix:"LLM_"`
}

func Parse() (*Config, error) {
	conf, err := env.ParseAsWithOptions[Config](env.Options{
		Prefix: "CORPUS_",
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &conf, nil
}
