package client

import (
	"net/http"
	"net/url"
	"sync"
)

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	mutex      sync.Mutex
}

func New(token string, funcs ...OptionFunc) *Client {
	opts := NewOptions(funcs...)
	return &Client{
		baseURL:    opts.BaseURL,
		token:      token,
		httpClient: opts.HTTPClient,
	}
}
