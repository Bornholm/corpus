package scraper

import (
	"context"
	"io"
)

type Scraper interface {
	Get(ctx context.Context, url string) (io.ReadCloser, error)
	Check(ctx context.Context, url string) (bool, error)
}
