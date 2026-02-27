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
	paramMaxRestarts = "max-restarts"
)

var (
	flagFilesystem = altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
		Name:    paramFilesystem,
		Aliases: []string{"f"},
		Value:   cli.NewStringSlice(),
		Usage:   "One or more filesystem DSN to watch",
	})
	flagMaxRestarts = altsrc.NewIntFlag(&cli.IntFlag{
		Name:  paramMaxRestarts,
		Value: 5,
		Usage: "Number of times a watcher can restart after an error before killing the whole command",
	})
)

func withWatchFlags(flags ...cli.Flag) []cli.Flag {
	return append([]cli.Flag{
		flagFilesystem,
		flagMaxRestarts,
	}, flags...)
}

func getFilesystems(ctx *cli.Context) ([]string, error) {
	filesystems := ctx.StringSlice(paramFilesystem)
	return filesystems, nil
}

func getMaxRestarts(ctx *cli.Context) (int, error) {
	maxRestarts := ctx.Int(paramMaxRestarts)
	return maxRestarts, nil
}
