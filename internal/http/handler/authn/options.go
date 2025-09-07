package authn

import "github.com/bornholm/corpus/internal/http/handler/authn/component"

type Provider = component.Provider

type Options struct {
	Providers   []component.Provider
	SessionName string
}

type OptionFunc func(opts *Options)

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		Providers:   make([]Provider, 0),
		SessionName: "corpus_auth",
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func WithProviders(providers ...Provider) OptionFunc {
	return func(opts *Options) {
		opts.Providers = providers
	}
}

func WithSessionName(sessionName string) OptionFunc {
	return func(opts *Options) {
		opts.SessionName = sessionName
	}
}
