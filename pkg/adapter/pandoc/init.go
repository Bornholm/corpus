package pandoc

import (
	"net/url"

	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/corpus/internal/setup"
)

func init() {
	setup.FileConverter.Register("pandoc", func(u *url.URL) (port.FileConverter, error) {
		return NewFileConverter(), nil
	})
}
