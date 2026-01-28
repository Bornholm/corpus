package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/http"
	"github.com/bornholm/corpus/internal/http/handler/mcp"
	"github.com/bornholm/corpus/internal/http/handler/metrics"
	"github.com/bornholm/corpus/internal/http/handler/webui"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/corpus/internal/http/handler/webui/pubshare"
	"github.com/bornholm/corpus/internal/http/middleware/authn"
	"github.com/pkg/errors"

	gohttp "net/http"
)

func NewHTTPServerFromConfig(ctx context.Context, conf *config.Config) (*http.Server, error) {
	api, err := getAPIHandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure api handler from config")
	}

	oidcAuthn, err := getOIDCAuthnHandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure authn oidc handler from config")
	}

	tokenAuthn, err := getTokenAuthnHandlerFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure authn token handler from config")
	}

	authnMiddleware := authn.Middleware(
		func(w gohttp.ResponseWriter, r *gohttp.Request) {
			// By default, redirect to OIDC login page if no user has been found
			gohttp.Redirect(w, r, "/auth/oidc/login", gohttp.StatusSeeOther)
		},
		tokenAuthn,
		oidcAuthn,
	)

	bridgeMiddleware, err := getBridgeMiddlewareFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not configure bridge middleware from config")
	}

	authChain := func(h gohttp.Handler) gohttp.Handler {
		return authnMiddleware(bridgeMiddleware(h))
	}

	assets := common.NewHandler()

	documentManager, err := getDocumentManager(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create index from config")
	}

	publicShareStore, err := getPublicShareStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create public share store from config")
	}

	pubshare := pubshare.NewHandler(documentManager, publicShareStore)

	options := []http.OptionFunc{
		http.WithAddress(conf.HTTP.Address),
		http.WithBaseURL(conf.HTTP.BaseURL),
		http.WithMount("/assets/", assets),
		http.WithMount("/shares/", pubshare),
		http.WithMount("/auth/oidc/", oidcAuthn),
		http.WithMount("/auth/token/", tokenAuthn),
		http.WithMount("/api/v1/", authChain(api)),
		http.WithMount("/metrics/", authChain(metrics.NewHandler())),
		http.WithMount("/mcp/", authChain(mcp.NewHandler(conf.HTTP.BaseURL, "/mcp", documentManager))),
	}

	llm, err := getLLMClientFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create llm client from config")
	}

	taskRunner, err := getTaskRunner(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create task runner from config")
	}

	if err := setupTaskHandlers(ctx, conf, taskRunner); err != nil {
		return nil, errors.Wrap(err, "could not setup task handlers from config")
	}

	userStore, err := getUserStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create user store from config")
	}

	documentStore, err := getDocumentStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.Wrap(err, "could not create document store from config")
	}

	webui := webui.NewHandler(documentManager, llm, taskRunner, userStore, documentStore, publicShareStore)

	options = append(options, http.WithMount("/", authChain(webui)))

	// Create HTTP server

	server := http.NewServer(options...)

	return server, nil
}
