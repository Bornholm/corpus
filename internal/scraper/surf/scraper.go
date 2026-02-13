package surf

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/bornholm/corpus/internal/scraper"
	"github.com/enetx/g"
	"github.com/enetx/surf"
	"github.com/pkg/errors"
)

type Scraper struct {
}

// Check implements scraper.Scraper.
func (s *Scraper) Check(ctx context.Context, url string) (bool, error) {
	client := s.getClient()
	resp := client.Get(g.String(url)).WithContext(ctx).Do()
	if resp.IsErr() {
		return false, errors.WithStack(resp.Err())
	}

	return resp.IsOk(), nil
}

// Get implements scraper.Scraper.
func (s *Scraper) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	client := s.getClient()
	resp := client.Get(g.String(url)).WithContext(ctx).Do()
	if resp.IsErr() {
		return nil, errors.WithStack(resp.Err())
	}

	return resp.Ok().Body.Reader, nil
}

func (s *Scraper) getClient() *surf.Client {
	builder := surf.NewClient().
		Builder()

	if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
		builder = builder.Proxy(g.String(proxy))
	}

	builder = builder.Impersonate().RandomOS().Chrome().
		Timeout(20*time.Second).
		Retry(2, 3).
		Session()

	return builder.Build().Unwrap()
}

func NewScraper() *Scraper {
	return &Scraper{}
}

var _ scraper.Scraper = &Scraper{}
