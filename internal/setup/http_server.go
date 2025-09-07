package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/http"
	"github.com/bornholm/corpus/internal/http/handler/mcp"
	"github.com/bornholm/corpus/internal/http/handler/metrics"
	"github.com/bornholm/corpus/internal/http/handler/webui"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/pkg/errors"
)

func NewHTTPServerFromConfig(ctx context.Context, conf *config.Config) (*http.Server, error) {
	api, err := getAPIHandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure api handler from config")
	}

	authn, err := getAuthnHandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure authn handler from config")
	}

	authnMiddleware := authn.Middleware()

	authzMiddleware, err := getAuthzMiddlewareFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure authz handler from config")
	}

	assets := common.NewHandler()

	documentManager, err := getDocumentManager(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	options := []http.OptionFunc{
		http.WithAddress(conf.HTTP.Address),
		http.WithBaseURL(conf.HTTP.BaseURL),
		http.WithMount("/assets/", assets),
		http.WithMount("/auth/", authn),
		http.WithMount("/api/v1/", authnMiddleware(authzMiddleware(api))),
		http.WithMount("/metrics/", authnMiddleware(authzMiddleware(metrics.NewHandler()))),
		http.WithMount("/mcp/", authnMiddleware(authzMiddleware(mcp.NewHandler(conf.HTTP.BaseURL, "/mcp", documentManager)))),
	}

	if conf.WebUI.Enabled {
		llm, err := getLLMClientFromConfig(ctx, conf)
		if err != nil {
			return nil, errors.Wrap(err, "could not create llm client from config")
		}

		taskRunner, err := getTaskRunner(ctx, conf)
		if err != nil {
			return nil, errors.Wrap(err, "could not create task runner from config")
		}

		webui := webui.NewHandler(documentManager, llm, taskRunner)

		options = append(options, http.WithMount("/", authnMiddleware(authzMiddleware(webui))))
	}

	// Create HTTP server

	server := http.NewServer(options...)

	return server, nil
}
