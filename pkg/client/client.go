package client

import (
	"net/http"
	"net/url"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	semaphore  chan struct{}
}

func New(funcs ...OptionFunc) *Client {
	opts := NewOptions(funcs...)
	return &Client{
		baseURL:    opts.BaseURL,
		httpClient: opts.HTTPClient,
		semaphore:  make(chan struct{}, opts.Parallelism),
	}
}
