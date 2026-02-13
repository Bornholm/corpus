package chromedp

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/bornholm/corpus/internal/scraper"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"

	cu "github.com/Davincible/chromedp-undetected"
)

type Scraper struct {
	chromeCtx    context.Context
	cancelChrome context.CancelFunc
}

// Check implements scraper.Scraper.
func (s *Scraper) Check(ctx context.Context, url string) (bool, error) {
	err := chromedp.Run(s.chromeCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
}

// Get implements scraper.Scraper.
func (s *Scraper) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	var html string

	err := chromedp.Run(s.chromeCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return errors.WithStack(err)
			}
			res, err := dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			if err != nil {
				return errors.WithStack(err)
			}

			html = res

			return errors.WithStack(err)
		}),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return io.NopCloser(bytes.NewBufferString(html)), nil
}

func (s *Scraper) Close() {
	s.cancelChrome()
}

func NewScraper(headless bool) (*Scraper, error) {
	options := []cu.Option{}
	if headless {
		options = append(options, cu.WithHeadless())
	}

	if httpProxy := os.Getenv("HTTP_PROXY"); httpProxy != "" {
		options = append(options, cu.WithChromeFlags(chromedp.ProxyServer(httpProxy)))
	}

	chromeCtx, cancelChrome, err := cu.New(cu.NewConfig(options...))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &Scraper{
		chromeCtx:    chromeCtx,
		cancelChrome: cancelChrome,
	}, nil
}

var _ scraper.Scraper = &Scraper{}
