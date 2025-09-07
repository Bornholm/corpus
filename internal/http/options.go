package http

import (
	"net/http"
)

type Options struct {
	Address string
	BaseURL string
	Mounts  map[string]http.Handler
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
