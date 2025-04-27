package common

import (
	"net/url"

	"github.com/bornholm/corpus/pkg/client"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

const (
	paramServer = "server"
)

var (
	flagServer = altsrc.NewStringFlag(&cli.StringFlag{
		Name:    paramServer,
		Aliases: []string{"s"},
		Value:   "http://localhost:3002",
		Usage:   "Corpus server base url",
	})
)

func WithCommonFlags(flags ...cli.Flag) []cli.Flag {
	return append([]cli.Flag{
		flagServer,
	}, flags...)
}

func GetCorpusClient(ctx *cli.Context) (*client.Client, error) {
	rawServerURL := ctx.String(paramServer)

	serverURL, err := url.Parse(rawServerURL)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return client.New(
		client.WithBaseURL(serverURL),
	), nil
}
