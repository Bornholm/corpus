package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"

	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/desktop"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/internal/setup"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
	"github.com/zserge/lorca"

	// Adapters
	_ "github.com/bornholm/corpus/internal/adapter/genai"
	_ "github.com/bornholm/corpus/internal/adapter/memory"
	_ "github.com/bornholm/corpus/internal/adapter/pandoc"

	// GenAI text extractors
	_ "github.com/bornholm/genai/extract/provider/marker"
	_ "github.com/bornholm/genai/extract/provider/mistral"
)

var (
	logLevel int  = int(slog.LevelInfo)
	noUpdate bool = false
)

func init() {
	flag.IntVar(&logLevel, "log-level", logLevel, "log level")
	flag.BoolVar(&noUpdate, "no-update", noUpdate, "disable self update")
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !noUpdate && build.LongVersion != "unknown" {
		updated, err := update(ctx, build.LongVersion)
		if err != nil {
			slog.WarnContext(ctx, "could not update the app", slogx.Error(err))
		} else if updated {
			if err := restartSelf(ctx); err != nil {
				slog.WarnContext(ctx, "could not restart the app automatically", slogx.Error(err))
			} else {
				os.Exit(0)
			}
		}
	}

	logger := slog.New(log.ContextHandler{
		Handler: slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     slog.Level(logLevel),
			AddSource: true,
		}),
	})

	slog.SetDefault(logger)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	go func() {
		slog.InfoContext(ctx, "use ctrl+c to interrupt")
		<-sig
		cancel()
	}()

	app, err := lorca.New(
		lorca.WithWindowSize(800, 900),
		lorca.WithAdditionalCustomArgs(
			"--guest",
			fmt.Sprintf("--user-agent=%s/%s", desktop.UserAgentPrefix, build.ShortVersion),
		),
	)
	if err != nil {
		slog.Error("could not run app", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	defer app.Close()

	server, err := setup.NewDesktopServer(ctx)
	if err != nil {
		slog.Error("could not create desktop server", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	var listener net.Listener

	retries := 0
	for {
		port, total := desktop.NextPort()

		listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			slog.Error("could not listen", slog.Any("error", errors.WithStack(err)))
			if retries > total {
				os.Exit(1)
			}
			retries++
			continue
		}

		break
	}

	go func() {
		if err := server.Serve(listener); err != nil {
			slog.Error("could not serve application", slog.Any("error", errors.WithStack(err)))
			os.Exit(1)
		}
	}()

	serverAddr := listener.Addr()

	if err := app.Load(fmt.Sprintf("http://%s", serverAddr.String())); err != nil {
		slog.Error("could not serve application", slog.Any("error", errors.WithStack(err)))
		os.Exit(1)
	}

	<-app.Done()
}
