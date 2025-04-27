package common

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"regexp"

	"github.com/Bornholm/amatl/pkg/resolver"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"gopkg.in/yaml.v2"
)

func NewResolverSourceFromFlagFunc(flag string) func(cCtx *cli.Context) (altsrc.InputSourceContext, error) {
	return func(cCtx *cli.Context) (altsrc.InputSourceContext, error) {
		if urlStr := cCtx.String(flag); urlStr != "" {
			return NewResolvedInputSource(cCtx.Context, urlStr)
		}

		return altsrc.NewMapInputSource("", map[any]any{}), nil
	}
}

func NewResolvedInputSource(ctx context.Context, urlStr string) (altsrc.InputSourceContext, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse url '%s'", urlStr)
	}

	reader, err := resolver.Resolve(ctx, url)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer func() {
		if err := reader.Close(); err != nil {
			panic(errors.WithStack(err))
		}
	}()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ext := filepath.Ext(url.Path)
	switch ext {
	case ".json":
		fallthrough
	case ".yaml":
		fallthrough
	case ".yml":
		var values map[any]any

		if err := yaml.Unmarshal(data, &values); err != nil {
			return nil, errors.WithStack(err)
		}

		values, err = rewriteRelativeURL(url, values)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return altsrc.NewMapInputSource(urlStr, values), nil

	default:
		return nil, errors.Errorf("no parser associated with '%s' file extension", ext)
	}

}

func rewriteRelativeURL(fromURL *url.URL, values map[any]any) (map[any]any, error) {
	fromURL.Path = filepath.Dir(fromURL.Path)

	absPath, err := filepath.Abs(fromURL.Path)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	fromURL.Path = absPath

	for key, rawValue := range values {
		value, ok := rawValue.(string)
		if !ok {
			continue
		}

		switch {
		case isURL(value):
			continue

		case isPath(value):
			if filepath.IsAbs(value) {
				continue
			}

			values[key] = fromURL.JoinPath(value).String()
			continue
		}

	}

	return values, nil
}

var filepathRegExp = regexp.MustCompile(`^(?i)(?:\/[^\/]+)+\/?[^\s]+(?:\.[^\s]+)+|[^\s]+(?:\.[^\s]+)+$`)

func isPath(str string) bool {
	return filepathRegExp.MatchString(str)
}

func isURL(str string) bool {
	_, err := url.ParseRequestURI(str)
	return err == nil
}
