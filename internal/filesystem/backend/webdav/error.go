package webdav

import (
	"errors"
	"io/fs"
	"os"

	"github.com/studio-b12/gowebdav"
)

func wrapWebDavError(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr
	}

	var statusErr gowebdav.StatusError
	if errors.As(err, &statusErr) {
		return &statusErr
	}

	return err
}

func isWebDavErr(err error, op string, status int) bool {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		if pathErr.Op != op {
			return false
		}

		var statusErr gowebdav.StatusError
		if errors.As(pathErr.Err, &statusErr) {
			return statusErr.Status == status
		}

		return false
	}

	return false
}
