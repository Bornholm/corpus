package libreoffice

import (
	"net/url"

	"github.com/bornholm/corpus/internal/adapter/pandoc"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/setup"
)

func init() {
	setup.FileConverter.Register("libreoffice+pandoc", func(u *url.URL) (port.FileConverter, error) {
		return NewFileConverter(
			pandoc.NewFileConverter(),
		), nil
	})
}
