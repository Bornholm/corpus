package scraper

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

type HTTPScraper struct {
	client *http.Client
}

// Check implements scraper.Scraper.
func (s *HTTPScraper) Check(ctx context.Context, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, errors.WithStack(err)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return false, errors.WithStack(err)
	}

	defer res.Body.Close()

	ok := res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest

	return ok, nil
}

// Get implements scraper.Scraper.
func (s *HTTPScraper) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ok := res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusBadRequest

	if !ok {
		defer res.Body.Close()

		body, err := io.ReadAll(io.LimitReader(res.Body, 4e+6)) // Restrict to 4MB
		if err != nil {
			slog.ErrorContext(ctx, "could not read response body", slogx.Error(errors.WithStack(err)))
		}

		return nil, errors.Errorf("unexpected response http status %d (%s):\n%s", res.StatusCode, res.Status, body)
	}

	return res.Body, nil
}

func NewHTTPScraper(client *http.Client) *HTTPScraper {
	return &HTTPScraper{
		client: client,
	}
}

var _ Scraper = &HTTPScraper{}
