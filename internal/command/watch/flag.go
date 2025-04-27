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
	paramFilesystem = "filesystem"
)

var (
	flagFilesystem = altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
		Name:    paramFilesystem,
		Aliases: []string{"f"},
		Value:   cli.NewStringSlice(),
		Usage:   "One or more filesystem DSN to watch",
	})
)

func withWatchFlags(flags ...cli.Flag) []cli.Flag {
	return append([]cli.Flag{
		flagFilesystem,
	}, flags...)
}

func getFilesystems(ctx *cli.Context) ([]string, error) {
	filesystems := ctx.StringSlice(paramFilesystem)
	return filesystems, nil
}
