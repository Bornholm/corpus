package watch

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	// Register resolver schemes

	_ "github.com/Bornholm/amatl/pkg/resolver/file"
	_ "github.com/Bornholm/amatl/pkg/resolver/http"
	_ "github.com/Bornholm/amatl/pkg/resolver/stdin"
)

const (
	paramFilesystem  = "filesystem"
	paramConcurrency = "concurrency"
)

var (
	flagFilesystem = altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
		Name:    paramFilesystem,
		Aliases: []string{"f"},
		Value:   cli.NewStringSlice(),
		Usage:   "One or more filesystem DSN to watch",
	})
	flagConcurrency = altsrc.NewIntFlag(&cli.IntFlag{
		Name:  paramConcurrency,
		Value: 5,
		Usage: "Maximum number of concurrent operations to execute (by filesystem)",
	})
)

func withWatchFlags(flags ...cli.Flag) []cli.Flag {
	return append([]cli.Flag{
		flagFilesystem,
		flagConcurrency,
	}, flags...)
}

func getFilesystems(ctx *cli.Context) ([]string, error) {
	filesystems := ctx.StringSlice(paramFilesystem)
	return filesystems, nil
}

func getConcurrency(ctx *cli.Context) (int, error) {
	concurrency := ctx.Int(paramConcurrency)
	return concurrency, nil
}
