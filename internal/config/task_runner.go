package config

type TaskRunner struct {
	URI string `env:"URI,expand" envDefault:"memory://taskrunner?parallelism=10&cleanupInterval=10m&cleanupDelay=1h"`
}
