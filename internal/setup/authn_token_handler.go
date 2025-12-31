package setup

import (
	"context"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/http/middleware/authn/token"
	"github.com/pkg/errors"
)

func getTokenAuthnHandlerFromConfig(ctx context.Context, conf *config.Config) (*token.Handler, error) {
	sessionStore, err := getSessionStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	userStore, err := getUserStoreFromConfig(ctx, conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	handler := token.NewHandler(sessionStore, userStore)

	return handler, nil
}
