package watch

import (
	"context"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/redmatter/go-globre/v2"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	"github.com/bornholm/corpus/internal/command/common"
	"github.com/bornholm/corpus/internal/filesystem"
	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/bornholm/corpus/internal/log"

	// Filesystem backends

	_ "github.com/bornholm/corpus/internal/filesystem/backend/ftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/git"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/local"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/minio"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/sftp"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/smb"
	_ "github.com/bornholm/corpus/internal/filesystem/backend/webdav"
)

func Command() *cli.Command {
	flags := common.WithCommonFlags(
		withWatchFlags()...,
	)
	return &cli.Command{
		Name:   "watch",
		Usage:  "Watch one or more filesystems and automatically index files on change",
		Flags:  flags,
		Before: altsrc.InitInputSourceWithContext(flags, common.NewResolverSourceFromFlagFunc("config")),
		Action: func(ctx *cli.Context) error {
			filesystems, err := getFilesystems(ctx)
			if err != nil {
				return errors.Wrap(err, "could not retrieve filesystems")
			}

			concurrency, err := getConcurrency(ctx)
			if err != nil {
				return errors.Wrap(err, "could not retrieve concurrency")
			}

			client, err := common.GetCorpusClient(ctx)
			if err != nil {
				return errors.Wrap(err, "could not retrieve corpus client")
			}

			sharedCtx, sharedCancel := context.WithCancel(ctx.Context)
			defer sharedCancel()

			var wg sync.WaitGroup

			wg.Add(len(filesystems))

			for _, f := range filesystems {
				dsn, err := url.Parse(f)
				if err != nil {
					return errors.WithStack(err)
				}

				collections, err := getCorpusCollections(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve collections from dsn '%s'", dsn)
				}

				source, err := getCorpusSource(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve source from dsn '%s'", dsn)
				}

				eTagType, err := getCorpusETagType(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve etag type from dsn '%s'", dsn)
				}

				watchOptions, err := getWatchOptions(dsn)
				if err != nil {
					return errors.Wrapf(err, "could not retrieve watch options from dsn '%s'", dsn)
				}

				b, err := backend.New(dsn.String())
				if err != nil {
					return errors.Wrapf(err, "could not create filesystem backend from dsn '%s'", dsn)
				}

				go func(b filesystem.Backend, dsn *url.URL, collections []string, source *url.URL, watchOptions []filesystem.WatchOptionFunc, eTagType ETagType) {
					defer wg.Done()
					defer sharedCancel()

					watchCtx := log.WithAttrs(sharedCtx, slog.String("filesystem", scrubbedURL(dsn)))

					indexer := &filesystemIndexer{
						collections: collections,
						client:      client,
						backend:     b,
						source:      source,
						eTagType:    eTagType,
						semaphore:   make(chan struct{}, concurrency),
					}

					if err := indexer.Watch(watchCtx, watchOptions...); err != nil {
						slog.ErrorContext(watchCtx, "could not watch filesystem", slog.Any("error", errors.WithStack(err)))
					}
				}(b, dsn, collections, source, watchOptions, eTagType)
			}

			wg.Wait()

			return nil
		},
	}
}

const (
	paramCorpusCollections = "corpusCollections"
)

func getCorpusCollections(dsn *url.URL) ([]string, error) {
	query := dsn.Query()

	collections := make([]string, 0)

	if rawCollections := query.Get(paramCorpusCollections); rawCollections != "" {
		colls := strings.Split(rawCollections, ",")
		collections = append(collections, colls...)
		query.Del(paramCorpusCollections)
	}

	dsn.RawQuery = query.Encode()

	return collections, nil
}

const (
	paramCorpusSource = "corpusSource"
)

func getCorpusSource(dsn *url.URL) (*url.URL, error) {
	query := dsn.Query()

	if rawSource := query.Get(paramCorpusSource); rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		query.Del(paramCorpusSource)
		dsn.RawQuery = query.Encode()

		return source, nil
	}

	return nil, nil
}

const (
	paramCorpusETag = "corpusEtag"
)

var availableETagTypes = []ETagType{
	ETagTypeModTime,
	ETagTypeSize,
}

func getCorpusETagType(dsn *url.URL) (ETagType, error) {
	query := dsn.Query()

	if rawETagType := query.Get(paramCorpusETag); rawETagType != "" {

		eTagType := ETagType(rawETagType)

		if !slices.Contains(availableETagTypes, eTagType) {
			return "", errors.Errorf("could not parse parameter '%s', unexpected value '%s'", paramCorpusETag, rawETagType)
		}

		query.Del(paramCorpusETag)
		dsn.RawQuery = query.Encode()

		return eTagType, nil
	}

	return ETagTypeModTime, nil
}

const (
	paramWatchRecursive = "watchRecursive"
	paramWatchInterval  = "watchInterval"
	paramWatchDirectory = "watchDirectory"
	paramWatchFilter    = "watchFilter"
)

func getWatchOptions(dsn *url.URL) ([]filesystem.WatchOptionFunc, error) {
	options := make([]filesystem.WatchOptionFunc, 0)

	query := dsn.Query()

	recursive := query.Get(paramWatchRecursive)
	switch recursive {
	case "false":
		options = append(options, filesystem.WithRecursive(false))
	case "true":
		fallthrough
	default:
		options = append(options, filesystem.WithRecursive(true))
	}

	query.Del(paramWatchRecursive)

	if rawInterval := query.Get(paramWatchInterval); rawInterval != "" {
		interval, err := time.ParseDuration(rawInterval)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse '%s' parameter", paramWatchInterval)
		}

		options = append(options, filesystem.WithInterval(interval))
	}

	query.Del(paramWatchInterval)

	if directory := query.Get(paramWatchDirectory); directory != "" {
		options = append(options, filesystem.WithDirectory(directory))
	}

	query.Del(paramWatchDirectory)

	if rawFilter := query.Get(paramWatchFilter); rawFilter != "" {
		pathRegExp := globre.RegexFromGlob(
			rawFilter,
			globre.ExtendedSyntaxEnabled(true),
			globre.GlobStarEnabled(true),
			globre.WithDelimiter('/'),
		)

		filter, err := regexp.Compile(pathRegExp)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse '%s' parameter", paramWatchFilter)
		}

		options = append(options, filesystem.WithFilter(filter))
	}

	query.Del(paramWatchFilter)

	dsn.RawQuery = query.Encode()

	return options, nil
}

func scrubbedURL(u *url.URL) string {
	if u.User != nil {
		u.User = url.UserPassword("***", "***")
	}
	return u.String()
}
