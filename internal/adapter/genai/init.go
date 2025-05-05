package genai

import (
	"context"
	"net/url"
	"strings"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/setup"
	"github.com/bornholm/genai/extract/provider"
	"github.com/bornholm/genai/extract/provider/marker"
	"github.com/bornholm/genai/extract/provider/mistral"
	"github.com/pkg/errors"
)

const (
	ParamExtensions = "extensions"
)

func init() {
	setup.FileConverter.Register(string(marker.Name), createFileConverter)
	setup.FileConverter.Register(string(mistral.Name), createFileConverter)
}

func createFileConverter(u *url.URL) (port.FileConverter, error) {
	dsn := u.JoinPath()

	query := dsn.Query()

	extensions := make([]string, 0)
	if rawExtensions := query.Get(ParamExtensions); rawExtensions != "" {
		extensions = strings.Split(rawExtensions, ",")
		query.Del(ParamExtensions)
		dsn.RawQuery = query.Encode()
	}

	client, err := provider.Create(
		context.Background(),
		provider.WithTextClientDSN(u.String()),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return NewFileConverter(client, extensions...), nil
}
