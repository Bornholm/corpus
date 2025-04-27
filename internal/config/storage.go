package config

type Storage struct {
	Database  Database       `envPrefix:"DATABASE_"`
	Bleve     BleveIndex     `envPrefix:"BLEVE_"`
	SQLiteVec SQLiteVecIndex `envPrefix:"SQLITEVEC_"`
}

type Database struct {
	DSN string `env:"DSN" envDefault:"data.sqlite"`
}

type SQLiteVecIndex struct {
	DSN string `env:"DSN" envDefault:"index.sqlite"`
}

type BleveIndex struct {
	DSN string `env:"DSN" envDefault:"index.bleve"`
}
