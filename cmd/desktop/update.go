package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"runtime"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/pkg/errors"
)

func update(ctx context.Context, version string) (bool, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return false, errors.WithStack(err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Arm:       0,
	})
	if err != nil {
		return false, errors.WithStack(err)
	}

	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug("Bornholm/corpus"))
	if err != nil {
		return false, errors.Errorf("error occurred while detecting version: %+v", errors.WithStack(err))
	}
	if !found {
		return false, errors.Errorf("latest version for %s/%s could not be found from github repository", runtime.GOOS, runtime.GOARCH)
	}

	slog.InfoContext(ctx, "latest stable version", "version", latest.Version())

	if latest.LessOrEqual(version) {
		slog.InfoContext(ctx, "current version is the latest", "version", version)
		return false, nil
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return false, errors.New("could not locate executable path")
	}

	if err := selfupdate.UpdateTo(ctx, latest.AssetURL, latest.AssetName, exe); err != nil {
		return false, errors.Errorf("error occurred while updating binary: %+v", errors.WithStack(err))
	}

	slog.InfoContext(ctx, "successfully updated to version", "version", latest.Version())

	return true, nil
}

func restartSelf(ctx context.Context) error {
	executable, err := selfupdate.ExecutablePath()
	if err != nil {
		return errors.New("could not locate executable path")
	}

	cmd := exec.Command(executable, os.Args[1:]...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return errors.WithStack(err)
	}

	slog.InfoContext(ctx, "new process started")

	return nil
}
