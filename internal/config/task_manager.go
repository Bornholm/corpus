package config

type TaskManager struct {
	URI string `env:"URI" envDefault:"memory://taskmanager?parallelism=10&cleanupInterval=10m&cleanupDelay=1h"`
}
