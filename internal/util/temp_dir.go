package util

import (
	"os"
	"sync"

	"github.com/pkg/errors"
)

var (
	createTempDirOnce sync.Once
	createTempDirErr  error
	tempDir           string
)

func TempDir() (string, error) {
	createTempDirOnce.Do(func() {
		tmp, err := os.MkdirTemp("", "corpus-*")
		if err != nil {
			createTempDirErr = errors.WithStack(err)
			return
		}

		tempDir = tmp
	})
	if createTempDirErr != nil {
		return "", errors.WithStack(createTempDirErr)
	}

	return tempDir, nil
}
