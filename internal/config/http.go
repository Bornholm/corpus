package config

import "time"

type HTTP struct {
	BaseURL string  `env:"BASE_URL,expand" envDefault:"/"`
	Address string  `env:"ADDRESS,expand" envDefault:":3002"`
	Authn   Authn   `envPrefix:"AUTHN_"`
	Session Session `envPrefix:"SESSION_"`
}
type Authn struct {
	Providers    AuthProviders     `envPrefix:"PROVIDERS_"`
	Whitelist    []string          `env:"WHITELIST" envSeparator:","`
	DefaultRole  string            `env:"DEFAULT_ROLE" envDefault:"reader"`
	RoleMappings map[string]string `env:"ROLE_MAPPINGS" envKeyValSeparator:":"`
}

type Session struct {
	Keys   []string `env:"KEYS" envSeparator:","`
	Cookie Cookie   `envPrefix:"COOKIE_"`
}

type Cookie struct {
	Path     string        `env:"PATH" envDefault:"/"`
	HTTPOnly bool          `env:"HTTP_ONLY" envDefault:"true"`
	Secure   bool          `env:"SECURE" envDefault:"false"`
	MaxAge   time.Duration `env:"MAX_AGE" enDefault:"24h"`
}

type AuthProviders struct {
	Google OAuth2Provider `envPrefix:"GOOGLE_"`
	Github OAuth2Provider `envPrefix:"GITHUB_"`
	Gitea  GiteaProvider  `envPrefix:"GITEA_"`
	OIDC   OIDCProvider   `envPrefix:"OIDC_"`
}

type OAuth2Provider struct {
	Key    string   `env:"KEY"`
	Secret string   `env:"SECRET"`
	Scopes []string `env:"SCOPES" envSeparator:"," envDefault:"profile,openid,email"`
}

type OIDCProvider struct {
	OAuth2Provider
	DiscoveryURL string `env:"DISCOVERY_URL"`
	Icon         string `env:"ICON"`
	Label        string `env:"LABEL"`
}

type GiteaProvider struct {
	OAuth2Provider
	TokenURL   string `env:"TOKEN_URL"`
	AuthURL    string `env:"AUTH_URL"`
	ProfileURL string `env:"PROFILE_URL"`
	Label      string `env:"LABEL"`
}
