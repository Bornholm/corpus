package reconciler

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (
	ETagTypeModTime = "modtime"
	ETagTypeSize    = "size"

	pathMarker        = "__PATH__"
	escapedPathMarker = "__ESCAPED_PATH__"
)

type Options struct {
	Directory      string
	SourceTemplate string
	SourceEmbedded bool
	ETagStrategy   string
}

type IndexJob struct {
	Path     string
	Filename string
	FileInfo os.FileInfo
	ETag     string
	Source   *url.URL
}

type DocumentDigestLister interface {
	ListDocumentDigests(ctx context.Context, sourcePrefix string, page int, pageSize int) ([]port.DocumentDigest, error)
}

func Reconcile(ctx context.Context, afs afero.Fs, lister DocumentDigestLister, opts Options) (toIndex []IndexJob, toDelete []model.DocumentID, err error) {
	directory := opts.Directory
	if directory == "" {
		directory = "."
	}

	etagStrategy := opts.ETagStrategy
	if etagStrategy == "" {
		etagStrategy = ETagTypeModTime
	}

	// 1. Fetch indexed digests for this source prefix
	indexed := make(map[string]port.DocumentDigest)

	prefix := getSourcePrefix(directory, opts)
	if prefix != "" {
		page := 0
		for {
			digests, err := lister.ListDocumentDigests(ctx, prefix, page, 500)
			if err != nil {
				return nil, nil, errors.WithStack(err)
			}
			for _, d := range digests {
				indexed[d.Source] = d
			}
			if len(digests) < 500 {
				break
			}
			page++
		}
	}

	slog.DebugContext(ctx, "fetched indexed digests", slog.Int("count", len(indexed)))

	// 2. Walk filesystem to enumerate local files
	local := make(map[string]IndexJob)

	err = afero.Walk(afs, directory, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			slog.WarnContext(ctx, "walk error", slog.String("path", path), slog.Any("error", walkErr))
			return nil
		}
		if info.IsDir() {
			return nil
		}

		src, err := getSource(path, opts)
		if err != nil || src == nil {
			return nil
		}

		etag, err := getETag(info, etagStrategy)
		if err != nil {
			return nil
		}

		local[src.String()] = IndexJob{
			Path:     path,
			Filename: filepath.Base(path),
			FileInfo: info,
			ETag:     etag,
			Source:   src,
		}
		return nil
	})
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "enumerated local files", slog.Int("count", len(local)))

	// 3. Determine files to add or update
	for srcStr, job := range local {
		digest, exists := indexed[srcStr]
		if !exists || digest.ETag != job.ETag {
			toIndex = append(toIndex, job)
		}
	}

	// 4. Determine orphans to delete
	for srcStr, digest := range indexed {
		if _, exists := local[srcStr]; !exists {
			toDelete = append(toDelete, digest.ID)
		}
	}

	slog.DebugContext(ctx, "reconciliation complete",
		slog.Int("toIndex", len(toIndex)),
		slog.Int("toDelete", len(toDelete)),
	)

	return toIndex, toDelete, nil
}

func GetSourcePrefix(directory string, opts Options) string {
	return getSourcePrefix(directory, opts)
}

func getSourcePrefix(directory string, opts Options) string {
	if opts.SourceEmbedded {
		return ""
	}
	src, err := getSource(directory, opts)
	if err != nil || src == nil {
		return ""
	}
	s := src.String()
	if !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
}

func getSource(path string, opts Options) (*url.URL, error) {
	if opts.SourceEmbedded {
		return nil, nil
	}

	cleanedPath := filepath.Clean(path)

	if opts.SourceTemplate != "" {
		escapedPath := url.QueryEscape(cleanedPath)
		rawSource := strings.ReplaceAll(opts.SourceTemplate, pathMarker, cleanedPath)
		rawSource = strings.ReplaceAll(rawSource, escapedPathMarker, escapedPath)
		src, err := url.Parse(rawSource)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return src, nil
	}

	src := &url.URL{
		Scheme: "file",
		Path:   cleanedPath,
	}
	return src, nil
}

func getETag(info os.FileInfo, strategy string) (string, error) {
	switch strategy {
	case ETagTypeModTime:
		return fmt.Sprintf("modtime-%d", info.ModTime().Unix()), nil
	case ETagTypeSize:
		return fmt.Sprintf("size-%d", info.Size()), nil
	default:
		return "", errors.Errorf("unexpected etag strategy '%s'", strategy)
	}
}
