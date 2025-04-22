package genai

import (
	"context"
	"net/url"
	"strings"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/setup"
	"github.com/bornholm/genai/llm/provider"
	"github.com/pkg/errors"
)

const (
	ParamScheme     = "_scheme"
	ParamAPIKey     = "_api_key"
	ParamProvider   = "_provider"
	ParamModel      = "_model"
	ParamExtensions = "_extensions"
)

func init() {
	setup.FileConverter.Register("genai", func(u *url.URL) (port.FileConverter, error) {
		opts := provider.ClientOptions{}

		query := u.Query()

		u.Scheme = "http"
		if query.Get(ParamScheme) != "" {
			u.Scheme = query.Get(ParamScheme)
			query.Del(ParamScheme)
		}

		if query.Get(ParamAPIKey) != "" {
			opts.APIKey = query.Get(ParamAPIKey)
			query.Del(ParamAPIKey)
		}

		if query.Get(ParamProvider) != "" {
			opts.Provider = provider.Name(query.Get(ParamProvider))
			query.Del(ParamProvider)
		}

		if query.Get(ParamModel) != "" {
			opts.Model = query.Get(ParamModel)
			query.Del(ParamModel)
		}

		extensions := make([]string, 0)
		if query.Get(ParamExtensions) != "" {
			extensions = strings.Split(query.Get(ParamExtensions), ",")
			query.Del(ParamExtensions)
		}

		u.RawQuery = query.Encode()

		opts.BaseURL = u.String()

		client, err := provider.Create(
			context.Background(),
			provider.WithExtractTextOptions(opts),
		)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return NewFileConverter(client, extensions...), nil
	})
}
