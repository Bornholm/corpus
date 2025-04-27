package client

import (
	"net/http"
	"net/url"
)

type Options struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
}

type OptionFunc func(opts *Options)

func WithBaseURL(baseURL *url.URL) OptionFunc {
	return func(opts *Options) {
		opts.BaseURL = baseURL
	}
}

func WithHTTPClient(httpClient *http.Client) OptionFunc {
	return func(opts *Options) {
		opts.HTTPClient = httpClient
	}
}

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		BaseURL: &url.URL{
			Scheme: "http",
			Host:   "localhost:3002",
		},
		HTTPClient: &http.Client{
			Timeout: 0,
		},
	}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}
