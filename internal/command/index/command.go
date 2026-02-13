package index

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bornholm/corpus/internal/command/common"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/scraper"
	"github.com/bornholm/corpus/internal/scraper/chromedp"
	"github.com/bornholm/corpus/internal/scraper/surf"
	"github.com/bornholm/corpus/pkg/client"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

const (
	flagInput       = "input"
	flagCollection  = "collection"
	flagConcurrency = "concurrency"
	flagScraper     = "scraper"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "index",
		Usage: "Index a list of files or URLs from a manifest file or stdin",
		Flags: common.WithCommonFlags(
			&cli.StringFlag{
				Name:     flagInput,
				Aliases:  []string{"i", "f"},
				Usage:    "Path to the manifest file containing the list of documents (use '-' for stdin)",
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:    flagCollection,
				Aliases: []string{"c"},
				Usage:   "Collection ID(s) to associate with the documents",
			},
			&cli.IntFlag{
				Name:    flagConcurrency,
				Value:   5,
				Usage:   "Number of concurrent uploads",
				EnvVars: []string{"CORPUS_CONCURRENCY"},
			},
			&cli.StringFlag{
				Name:     flagScraper,
				Usage:    "Scraper to use to download the HTTP resources (available: 'standard', 'chromedp', 'surf'; default: 'surf')",
				Value:    "surf",
				Required: false,
				EnvVars:  []string{"CORPUS_SCRAPER"},
			},
		),
		Action: func(cCtx *cli.Context) error {
			ctx := cCtx.Context
			logger := slog.Default()

			scraperType := cCtx.String(flagScraper)

			scraper, err := getScraper(scraperType)
			if err != nil {
				return errors.WithStack(err)
			}

			// 1. Initialisation du client
			corpusClient, err := common.GetCorpusClient(cCtx)
			if err != nil {
				return errors.Wrap(err, "could not create corpus client")
			}

			// 2. Récupération des IDs de collection
			collectionIDs := make([]model.CollectionID, 0)
			for _, id := range cCtx.StringSlice(flagCollection) {
				collectionIDs = append(collectionIDs, model.CollectionID(id))
			}

			if len(collectionIDs) == 0 {
				logger.WarnContext(ctx, "no collections specified, documents will be indexed but not organized")
			}

			// 3. Ouverture de la source (le manifest)
			inputPath := cCtx.String(flagInput)
			scanner, closer, err := resolveInputSource(ctx, inputPath, scraper)
			if err != nil {
				return errors.Wrap(err, "could not open input source")
			}
			defer closer.Close()

			// 4. Configuration de la concurrence
			concurrency := cCtx.Int(flagConcurrency)
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup

			// 5. Traitement ligne par ligne
			successCount := 0
			failCount := 0
			var mu sync.Mutex

			logger.InfoContext(ctx, "starting indexing", "source", inputPath, "concurrency", concurrency)

			startTime := time.Now()

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				wg.Add(1)
				sem <- struct{}{} // Acquire token

				go func(target string) {
					defer wg.Done()
					defer func() { <-sem }() // Release token

					taskID, skipped, err := processItem(ctx, corpusClient, target, collectionIDs, scraper)

					mu.Lock()
					defer mu.Unlock()

					if err != nil {
						failCount++
						logger.ErrorContext(ctx, "failed to index document",
							slog.String("target", target),
							slog.Any("error", err),
						)
					} else if skipped {
						logger.InfoContext(ctx, "document skipped (unchanged)",
							slog.String("target", target),
						)
					} else {
						successCount++
						logger.InfoContext(ctx, "document indexation task created successfully",
							slog.String("target", target),
							slog.String("task_id", string(taskID)),
						)
					}
				}(line)
			}

			if err := scanner.Err(); err != nil {
				return errors.Wrap(err, "error reading input source")
			}

			wg.Wait()

			duration := time.Since(startTime)
			logger.InfoContext(ctx, "indexing completed",
				slog.Int("success", successCount),
				slog.Int("failed", failCount),
				slog.Duration("duration", duration),
			)

			if failCount > 0 {
				return errors.New("some documents failed to index")
			}

			return nil
		},
	}
}

// resolveInputSource ouvre le fichier "manifest" ou stdin
func resolveInputSource(ctx context.Context, path string, scraper scraper.Scraper) (*bufio.Scanner, io.Closer, error) {
	var r io.ReadCloser
	var err error

	if path == "-" {
		r = os.Stdin
	} else if isURL(path) {
		res, err := scraper.Get(ctx, path)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		r = res
	} else {
		// Le manifest est un fichier local
		r, err = os.Open(path)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to open input file %s", path)
		}
	}

	return bufio.NewScanner(r), r, nil
}

// processItem traite une ressource et l'envoie au serveur avec son ETag
func processItem(ctx context.Context, c *client.Client, target string, collections []model.CollectionID, s scraper.Scraper) (model.TaskID, bool, error) {
	filename, reader, etag, err := resolveResource(ctx, target, s)
	if err != nil {
		return "", false, err
	}
	defer reader.Close()

	opts := []client.IndexOptionFunc{
		client.WithIndexCollections(collections...),
	}

	if etag != "" {
		opts = append(opts, client.WithIndexETag(etag))
	}

	if source, err := url.Parse(target); err == nil && source.Scheme != "" && !strings.HasPrefix(target, "/") {
		if etag != "" {
			documents, _, err := c.QueryDocuments(ctx, client.WithQueryDocumentsSource(source))
			if err != nil {
				return "", false, errors.WithStack(err)
			}

			if len(documents) > 0 && documents[0].ETag == etag {
				slog.InfoContext(ctx, "document already indexed, skipping", slog.String("etag", etag), slog.String("source", source.String()))
				return "", true, nil
			}
		}

		opts = append(opts, client.WithIndexSource(source))
	}

	slog.DebugContext(ctx, "indexing document", "filename", filename)

	task, err := c.Index(ctx, filename, reader, opts...)
	if err != nil {
		return "", false, errors.Wrapf(err, "client index failed for %s", filename)
	}

	return task.ID, false, nil
}

// resolveResource récupère le contenu et détermine l'ETag
func resolveResource(ctx context.Context, target string, s scraper.Scraper) (string, io.ReadCloser, string, error) {
	if isURL(target) {
		return resolveScrapedURL(ctx, target, s)
	}

	return resolveLocalFile(target)
}

func resolveLocalFile(path string) (string, io.ReadCloser, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, "", errors.Wrap(err, "file open failed")
	}

	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", nil, "", errors.Wrap(err, "hash calculation failed")
	}
	etag := hex.EncodeToString(h.Sum(nil))

	if _, err := f.Seek(0, 0); err != nil {
		return "", nil, "", errors.Wrap(err, "file seek failed")
	}

	stat, err := f.Stat()
	if err != nil {
		return "", nil, "", errors.Wrap(err, "file stat failed")
	}

	return stat.Name(), f, etag, nil
}

func resolveScrapedURL(ctx context.Context, target string, s scraper.Scraper) (string, io.ReadCloser, string, error) {
	content, err := s.Get(ctx, target)
	if err != nil {
		return "", nil, "", errors.Wrap(err, "scraping failed")
	}
	defer content.Close()

	var buf bytes.Buffer
	h := sha256.New()

	tee := io.TeeReader(content, h)

	if _, err := io.Copy(&buf, tee); err != nil {
		return "", nil, "", errors.Wrap(err, "reading content failed")
	}

	etag := hex.EncodeToString(h.Sum(nil))
	filename := getFilenameFromURL(target)

	return filename, io.NopCloser(&buf), etag, nil
}

func getFilenameFromURL(target string) string {
	u, _ := url.Parse(target)
	filename := "document"
	if items := strings.Split(u.Path, "/"); len(items) > 0 {
		if last := items[len(items)-1]; last != "" {
			filename = last
		}
	}

	// Use .pdf as default extension
	if filepath.Ext(filename) == "" {
		filename = filename + ".pdf"
	}

	return filename
}

func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func getScraper(scraperType string) (scraper.Scraper, error) {
	switch scraperType {
	case "surf":
		return surf.NewScraper(), nil
	case "standard":
		return scraper.NewHTTPScraper(http.DefaultClient), nil
	case "chromedp":
		var headless bool
		if rawHeadless := os.Getenv("HEADLESS"); rawHeadless != "" {
			headless, _ = strconv.ParseBool(rawHeadless)
		}

		scraper, err := chromedp.NewScraper(headless)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		return scraper, nil
	default:
		return nil, errors.Errorf("unknown scraper type '%s'", scraperType)
	}
}
