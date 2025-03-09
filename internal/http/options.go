package http

import "net/http"

type BasicAuth struct {
	Username string
	Password string
}

type Options struct {
	Address   string
	BaseURL   string
	BasicAuth *BasicAuth
	Mounts    map[string]http.Handler
}

type OptionFunc func(opts *Options)

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		Address: ":3002",
		BaseURL: "",
		Mounts:  map[string]http.Handler{},
	}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

func WithMount(prefix string, handler http.Handler) OptionFunc {
	return func(opts *Options) {
		opts.Mounts[prefix] = handler
	}
}

func WithBaseURL(baseURL string) OptionFunc {
	return func(opts *Options) {
		opts.BaseURL = baseURL
	}
}

func WithAddress(addr string) OptionFunc {
	return func(opts *Options) {
		opts.Address = addr
	}
}

func WithBasicAuth(username, password string) OptionFunc {
	return func(opts *Options) {
		opts.BasicAuth = &BasicAuth{
			Username: username,
			Password: password,
		}
	}
}
