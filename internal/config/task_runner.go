package config

// TaskRunner configure la file de tâches asynchrones.
//
// Drivers disponibles :
//   - memory://  — file en mémoire, perte des tâches au redémarrage (défaut)
//   - sqlite://  — file persistante SQLite/GORM, reprise après crash
//
// Paramètres communs (query string) :
//   - parallelism=N        — nombre de workers parallèles (défaut : 5)
//   - cleanupInterval=10m  — fréquence de purge des tâches terminées
//   - cleanupDelay=1h      — délai avant suppression d'une tâche terminée
//
// Exemple persistant : CORPUS_TASK_RUNNER_URI=sqlite://?parallelism=10
type TaskRunner struct {
	URI string `env:"URI,expand" envDefault:"memory://taskrunner?parallelism=5&cleanupInterval=10m&cleanupDelay=1h"`
}
