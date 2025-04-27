package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bornholm/corpus/internal/filesystem/testsuite"
	"github.com/pkg/errors"
)

func TestWatch(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	if err := os.RemoveAll(filepath.Join(cwd, "testdata/.local")); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	if err := os.MkdirAll(filepath.Join(cwd, "testdata/.local"), os.ModePerm); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	testsuite.TestWatch(t, "local://testdata/.local")
}
