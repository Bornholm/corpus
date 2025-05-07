package git

import (
	"log/slog"
	"os"
	"testing"

	"github.com/bornholm/corpus/internal/filesystem/testsuite"
)

func TestWatch(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Test skipped in CI environment because of non contained test")
		return
	}

	slog.SetLogLoggerLevel(slog.LevelDebug)

	testsuite.TestWatch(t, "git://github.com/Bornholm/corpus.git?gitPullInterval=2s")
}
