package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/http"
	"github.com/bornholm/corpus/internal/http/handler/webui"
	"github.com/pkg/errors"
)

func NewHTTPServerFromConfig(ctx context.Context, conf *config.Config) (*http.Server, error) {
	// Configure API handler
	api, err := getAPIHandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure api handler from config")
	}

	options := []http.OptionFunc{
		http.WithAddress(conf.HTTP.Address),
		http.WithBaseURL(conf.HTTP.BaseURL),
		http.WithMount("/api/v1/", api),
	}

	if conf.WebUI.Enabled {
		llm, err := getLLMClientFromConfig(ctx, conf)
		if err != nil {
			return nil, errors.Wrap(err, "could not create llm client from config")
		}

		documentManager, err := getDocumentManager(ctx, conf)
		if err != nil {
			return nil, errors.Wrap(err, "could not create index from config")
		}

		options = append(options, http.WithMount("/", webui.NewHandler(documentManager, llm)))
	}

	if conf.HTTP.Auth.Enabled {
		options = append(options, http.WithBasicAuth(conf.HTTP.Auth.Username, conf.HTTP.Auth.Password))
	}

	// Create HTTP server

	server := http.NewServer(options...)

	return server, nil
}
