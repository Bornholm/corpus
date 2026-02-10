package config

import "time"

type Storage struct {
	Database  Database       `envPrefix:"DATABASE_"`
	Bleve     BleveIndex     `envPrefix:"BLEVE_"`
	SQLiteVec SQLiteVecIndex `envPrefix:"SQLITEVEC_"`
}

type Database struct {
	DSN   string `env:"DSN,expand" envDefault:"data.sqlite"`
	Cache struct {
		Users     StoreCache `envPrefix:"USERS_"`
		Documents StoreCache `envPrefix:"DOCUMENTS_"`
	} `envPrefix:"CACHE_"`
}

type SQLiteVecIndex struct {
	DSN string `env:"DSN,expand" envDefault:"index.sqlite"`
}

type BleveIndex struct {
	DSN string `env:"DSN,expand" envDefault:"index.bleve"`
}

type StoreCache struct {
	Enabled bool          `env:"ENABLED,expand" envDefault:"true"`
	Size    int           `env:"SIZE,expand" envDefault:"25"`
	TTL     time.Duration `env:"TTL,expand" envDefault:"60m"`
}
