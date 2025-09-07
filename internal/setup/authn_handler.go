package setup

import (
	"context"
	"crypto/rand"

	"fmt"
	"net/http"

	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/http/handler/authn"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/pkg/errors"
)

func getAuthnHandlerFromConfig(ctx context.Context, conf *config.Config) (*authn.Handler, error) {
	keyPairs := make([][]byte, 0)
	if len(conf.HTTP.Session.Keys) == 0 {
		key, err := getRandomBytes(32)
		if err != nil {
			return nil, errors.Wrap(err, "could not generate cookie signing key")
		}

		keyPairs = append(keyPairs, key)
	} else {
		for _, k := range conf.HTTP.Session.Keys {
			keyPairs = append(keyPairs, []byte(k))
		}
	}

	sessionStore := sessions.NewCookieStore(keyPairs...)

	sessionStore.MaxAge(int(conf.HTTP.Session.Cookie.MaxAge.Seconds()))
	sessionStore.Options.Path = string(conf.HTTP.Session.Cookie.Path)
	sessionStore.Options.HttpOnly = bool(conf.HTTP.Session.Cookie.HTTPOnly)
	sessionStore.Options.Secure = conf.HTTP.Session.Cookie.Secure
	sessionStore.Options.SameSite = http.SameSiteLaxMode

	// Configure providers

	gothProviders := make([]goth.Provider, 0)
	providers := make([]authn.Provider, 0)

	if conf.HTTP.Authn.Providers.Google.Key != "" && conf.HTTP.Authn.Providers.Google.Secret != "" {
		googleProvider := google.New(
			string(conf.HTTP.Authn.Providers.Google.Key),
			string(conf.HTTP.Authn.Providers.Google.Secret),
			fmt.Sprintf("%s/auth/providers/google/callback", conf.HTTP.BaseURL),
			conf.HTTP.Authn.Providers.Google.Scopes...,
		)

		gothProviders = append(gothProviders, googleProvider)

		providers = append(providers, authn.Provider{
			ID:    googleProvider.Name(),
			Label: "Google",
			Icon:  "fa-google",
		})
	}

	if conf.HTTP.Authn.Providers.Github.Key != "" && conf.HTTP.Authn.Providers.Github.Secret != "" {
		githubProvider := github.New(
			string(conf.HTTP.Authn.Providers.Github.Key),
			string(conf.HTTP.Authn.Providers.Github.Secret),
			fmt.Sprintf("%s/auth/providers/github/callback", conf.HTTP.BaseURL),
			conf.HTTP.Authn.Providers.Github.Scopes...,
		)

		gothProviders = append(gothProviders, githubProvider)

		providers = append(providers, authn.Provider{
			ID:    githubProvider.Name(),
			Label: "Github",
			Icon:  "fa-github",
		})
	}

	if conf.HTTP.Authn.Providers.Gitea.Key != "" && conf.HTTP.Authn.Providers.Gitea.Secret != "" {
		giteaProvider := gitea.NewCustomisedURL(
			string(conf.HTTP.Authn.Providers.Gitea.Key),
			string(conf.HTTP.Authn.Providers.Gitea.Secret),
			fmt.Sprintf("%s/auth/providers/gitea/callback", conf.HTTP.BaseURL),
			string(conf.HTTP.Authn.Providers.Gitea.AuthURL),
			string(conf.HTTP.Authn.Providers.Gitea.TokenURL),
			string(conf.HTTP.Authn.Providers.Gitea.ProfileURL),
			conf.HTTP.Authn.Providers.Gitea.Scopes...,
		)

		gothProviders = append(gothProviders, giteaProvider)

		providers = append(providers, authn.Provider{
			ID:    giteaProvider.Name(),
			Label: string(conf.HTTP.Authn.Providers.Gitea.Label),
			Icon:  "fa-git-alt",
		})
	}

	if conf.HTTP.Authn.Providers.OIDC.Key != "" && conf.HTTP.Authn.Providers.OIDC.Secret != "" {
		oidcProvider, err := openidConnect.New(
			string(conf.HTTP.Authn.Providers.OIDC.Key),
			string(conf.HTTP.Authn.Providers.OIDC.Secret),
			fmt.Sprintf("%s/auth/providers/openid-connect/callback", conf.HTTP.BaseURL),
			string(conf.HTTP.Authn.Providers.OIDC.DiscoveryURL),
			conf.HTTP.Authn.Providers.OIDC.Scopes...,
		)
		if err != nil {
			return nil, errors.Wrap(err, "could not configure oidc provider")
		}

		gothProviders = append(gothProviders, oidcProvider)

		providers = append(providers, authn.Provider{
			ID:    oidcProvider.Name(),
			Label: string(conf.HTTP.Authn.Providers.OIDC.Label),
			Icon:  string(conf.HTTP.Authn.Providers.OIDC.Icon),
		})
	}

	goth.UseProviders(gothProviders...)
	gothic.Store = sessionStore

	opts := []authn.OptionFunc{
		authn.WithProviders(providers...),
	}

	handler := authn.NewHandler(
		sessionStore,
		opts...,
	)

	return handler, nil
}

func getRandomBytes(n int) ([]byte, error) {
	data := make([]byte, n)

	read, err := rand.Read(data)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if read != n {
		return nil, errors.Errorf("could not read %d bytes", n)
	}

	return data, nil
}
