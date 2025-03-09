package config

type Storage struct {
	Database Database `envPrefix:"DATABASE_"`
	Index    Index    `envPrefix:"INDEX_"`
}

type Database struct {
	DSN string `env:"DSN" envDefault:"data.sqlite"`
}

type Index struct {
	DSN string `env:"DSN" envDefault:"index.bleve"`
}
