package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

func (c *Client) request(method string, path string, header http.Header, body io.Reader, result io.Writer) error {
	url := c.baseURL.JoinPath("/api/v1", path)

	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return errors.WithStack(err)
	}

	req.Header = header

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

func (c *Client) jsonRequest(method string, path string, header http.Header, body io.Reader, result any) error {
	var buff bytes.Buffer

	if err := c.request(method, path, header, body, &buff); err != nil {
		return errors.WithStack(err)
	}

	if err := json.Unmarshal(buff.Bytes(), result); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
