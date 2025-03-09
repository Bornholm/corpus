package crypto

import (
	"crypto/rand"

	"github.com/pkg/errors"
)

func RandomBytes(size int) ([]byte, error) {
	data := make([]byte, size)

	read, err := rand.Read(data)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if read != size {
		return nil, errors.New("unexpected number of read bytes")
	}

	return data, nil
}
