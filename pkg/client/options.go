package client

import (
	"net/http"
	"net/url"
)

type Options struct {
	BaseURL     *url.URL
	HTTPClient  *http.Client
	Parallelism int
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

func WithParallelism(parallelism int) OptionFunc {
	return func(opts *Options) {
		opts.Parallelism = parallelism
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
			Transport: &RateLimitTransport{
				Base:       http.DefaultTransport,
				MaxRetries: 3,
			},
		},
		Parallelism: 5,
	}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}
