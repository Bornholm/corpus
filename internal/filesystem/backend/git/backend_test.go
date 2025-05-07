package git

import (
	"log/slog"
	"testing"

	"github.com/bornholm/corpus/internal/filesystem/testsuite"
)

func TestWatch(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	testsuite.TestWatch(t, "git://github.com/Bornholm/corpus.git?gitPullInterval=2s")
}
