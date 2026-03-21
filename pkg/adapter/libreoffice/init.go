package libreoffice

import (
	"net/url"

	"github.com/bornholm/corpus/pkg/adapter/pandoc"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/corpus/internal/setup"
)

func init() {
	setup.FileConverter.Register("libreoffice+pandoc", func(u *url.URL) (port.FileConverter, error) {
		return NewFileConverter(
			pandoc.NewFileConverter(),
		), nil
	})
}
