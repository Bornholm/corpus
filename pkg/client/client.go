package client

import (
	"net/http"
	"net/url"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func New(funcs ...OptionFunc) *Client {
	opts := NewOptions(funcs...)
	return &Client{
		baseURL:    opts.BaseURL,
		httpClient: opts.HTTPClient,
	}
}
