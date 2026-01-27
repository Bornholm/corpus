package client

import (
	"net/http"
	"net/url"
)

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	semaphore  chan struct{}
}

func New(token string, funcs ...OptionFunc) *Client {
	opts := NewOptions(funcs...)
	return &Client{
		baseURL:    opts.BaseURL,
		token:      token,
		httpClient: opts.HTTPClient,
		semaphore:  make(chan struct{}, opts.Parallelism),
	}
}
