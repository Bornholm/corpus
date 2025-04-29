package command

import (
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/log"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Main(name string, usage string, commands ...*cli.Command) {
	app := &cli.App{
		Name:     name,
		Usage:    usage,
		Commands: commands,
		Version:  build.LongVersion,
		Before: func(ctx *cli.Context) error {
			workdir := ctx.String("workdir")
			// Switch to new working directory if defined
			if workdir != "" {
				if err := os.Chdir(workdir); err != nil {
					return errors.Wrap(err, "could not change working directory")
				}
			}

			logLevel := ctx.String("log-level")
			slogLevel := slog.LevelWarn

			switch logLevel {
			case "debug":
				slogLevel = slog.LevelDebug
			case "info":
				slogLevel = slog.LevelInfo
			case "warn":
				slogLevel = slog.LevelWarn
			case "error":
				slogLevel = slog.LevelError
			}

			logger := slog.New(log.ContextHandler{
				Handler: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level:     slog.Level(slogLevel),
					AddSource: true,
				}),
			})

			slog.SetDefault(logger)

			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				EnvVars: []string{"CORPUS_CLI_CONFIG"},
				Aliases: []string{"c"},
				Usage:   "configuration file to use",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				EnvVars: []string{"CORPUS_CLI_DEBUG"},
				Usage:   "Toggle debug mode",
			},
			&cli.StringFlag{
				Name:    "workdir",
				Value:   "",
				EnvVars: []string{"CORPUS_CLI_WORKDIR"},
				Usage:   "The working directory",
			},
			&cli.StringFlag{
				Name:    "log-level",
				EnvVars: []string{"CORPUS_CLI_LOG_LEVEL"},
				Usage:   "Set logging level",
				Value:   "info",
			},
		},
	}

	app.ExitErrHandler = func(ctx *cli.Context, err error) {
		if err == nil {
			return
		}

		debug := ctx.Bool("debug")

		if !debug {
			slog.ErrorContext(ctx.Context, err.Error())
		} else {
			slog.ErrorContext(ctx.Context, fmt.Sprintf("%+v", err))
		}
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
