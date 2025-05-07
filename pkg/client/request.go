package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

func (c *Client) request(ctx context.Context, method string, path string, header http.Header, body io.Reader, result io.Writer) error {
	url, err := url.Parse(path)
	if err != nil {
		return errors.WithStack(err)
	}

	url.Scheme = c.baseURL.Scheme
	url.Host = c.baseURL.Host
	url.User = c.baseURL.User
	url.Path = c.baseURL.JoinPath("/api/v1", url.Path).Path

	slogAttrs := []any{
		slog.String("method", method),
		slog.String("path", url.Path),
		slog.String("host", url.Host),
	}
	if url.User != nil {
		slogAttrs = append(slogAttrs, slog.String("username", url.User.Username()))
	}

	slog.DebugContext(ctx, "new client request", slogAttrs...)

	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return errors.WithStack(err)
	}

	req = req.WithContext(ctx)

	if header != nil {
		for k, v := range header {
			req.Header[k] = v
		}
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}

	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		return errors.Errorf("unexpected response code %d (%s)", res.StatusCode, res.Status)
	}

	if _, err := io.Copy(result, res.Body); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *Client) jsonRequest(ctx context.Context, method string, path string, header http.Header, body io.Reader, result any) error {
	var buff bytes.Buffer

	if err := c.request(ctx, method, path, header, body, &buff); err != nil {
		return errors.WithStack(err)
	}

	if err := json.Unmarshal(buff.Bytes(), result); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
