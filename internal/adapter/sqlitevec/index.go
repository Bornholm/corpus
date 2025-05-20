package sqlitevec

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/text"
	"github.com/bornholm/genai/llm"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

type Index struct {
	maxWords int
	getConn  func(ctx context.Context) (*sqlite3.Conn, error)
	llm      llm.Client
	model    string
	lock     sync.Mutex
}

// DeleteByID implements port.Index.
func (i *Index) DeleteByID(ctx context.Context, ids ...model.SectionID) error {
	err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		stmt, _, err := conn.Prepare("DELETE FROM embeddings WHERE section_id IN ( SELECT value FROM json_each(?) );")
		if err != nil {
			return errors.WithStack(err)
		}

		defer stmt.Close()

		jsonIDs, err := json.Marshal(ids)
		if err != nil {
			return errors.WithStack(err)
		}

		if err := stmt.BindBlob(1, jsonIDs); err != nil {
			return errors.WithStack(err)
		}

		if err := stmt.Exec(); err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// DeleteBySource implements port.Index.
func (i *Index) DeleteBySource(ctx context.Context, source *url.URL) error {
	err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		stmt, _, err := conn.Prepare("DELETE FROM embeddings WHERE source = ?;")
		if err != nil {
			return errors.WithStack(err)
		}

		defer stmt.Close()

		if err := stmt.BindText(1, source.String()); err != nil {
			return errors.WithStack(err)
		}

		if err := stmt.Exec(); err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (i *Index) deleteBySource(ctx context.Context, conn *sqlite3.Conn, source *url.URL) error {
	stmt, _, err := conn.Prepare("DELETE FROM embeddings WHERE source = ?;")
	if err != nil {
		return errors.WithStack(err)
	}

	defer stmt.Close()

	if err := stmt.BindText(1, source.String()); err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.Exec(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Index implements port.Index.
func (i *Index) Index(ctx context.Context, document model.Document, funcs ...port.IndexOptionFunc) error {
	opts := port.NewIndexOptions(funcs...)

	err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		source := document.Source()

		if err := i.deleteBySource(ctx, conn, source); err != nil {
			return errors.WithStack(err)
		}

		totalSections := model.CountSections(document)

		slog.DebugContext(ctx, "total sections", slog.Int("total", totalSections))

		totalIndexed := 0
		onSectionIndexed := func() {
			if opts.OnProgress == nil {
				return
			}

			totalIndexed++
			progress := float32(totalIndexed) / float32(totalSections)
			opts.OnProgress(progress)
		}

		for _, s := range document.Sections() {
			if err := i.indexSection(ctx, conn, s, onSectionIndexed); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (i *Index) indexSection(ctx context.Context, conn *sqlite3.Conn, section model.Section, onSectionIndexed func()) (err error) {
	content, err := section.Content()
	if err != nil {
		return errors.WithStack(err)
	}

	truncated := text.MiddleOut(string(content), i.maxWords, "")

	if len(truncated) == 0 {
		slog.DebugContext(ctx, "ignoring empty section", slog.String("sectionID", string(section.ID())))
		return nil
	}

	slog.DebugContext(ctx, "indexing section", slog.String("sectionID", string(section.ID())), slog.Int("sectionSize", len(truncated)))

	res, err := i.llm.Embeddings(ctx, truncated)
	if err != nil {
		return errors.WithStack(err)
	}

	stmt, _, err := conn.Prepare("INSERT INTO embeddings ( source, section_id, embeddings ) VALUES (?, ?, vec_normalize(vec_slice(?, 0, 256))) RETURNING id;")
	if err != nil {
		return errors.WithStack(err)
	}

	defer stmt.Close()

	if err := stmt.BindText(1, section.Document().Source().String()); err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.BindText(2, string(section.ID())); err != nil {
		return errors.WithStack(err)
	}

	embeddings, err := sqlite_vec.SerializeFloat32(toFloat32(res.Embeddings()[0]))
	if err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.BindBlob(3, embeddings); err != nil {
		return errors.WithStack(err)
	}

	stmt.Step()

	if err := stmt.Err(); err != nil {
		return errors.WithStack(err)
	}

	embeddingsID := stmt.ColumnInt(0)

	if err := stmt.Close(); err != nil {
		return errors.WithStack(err)
	}

	for _, coll := range section.Document().Collections() {
		if err := i.insertCollection(ctx, conn, embeddingsID, coll.ID()); err != nil {
			return errors.WithStack(err)
		}
	}

	defer onSectionIndexed()

	for _, s := range section.Sections() {
		if err := i.indexSection(ctx, conn, s, onSectionIndexed); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (i *Index) insertCollection(ctx context.Context, conn *sqlite3.Conn, embeddingsID int, collectionID model.CollectionID) error {
	deleteStmt, _, err := conn.Prepare("DELETE FROM embeddings_collections WHERE embeddings_id = ? and collection_id = ?;")
	if err != nil {
		return errors.WithStack(err)
	}

	defer deleteStmt.Close()

	if err := deleteStmt.BindInt(1, embeddingsID); err != nil {
		return errors.WithStack(err)
	}

	if err := deleteStmt.BindText(2, string(collectionID)); err != nil {
		return errors.WithStack(err)
	}

	if err := deleteStmt.Exec(); err != nil {
		return errors.WithStack(err)
	}

	deleteStmt.Close()

	insertStmt, _, err := conn.Prepare("INSERT INTO embeddings_collections ( embeddings_id, collection_id ) VALUES (?, ?);")
	if err != nil {
		return errors.WithStack(err)
	}

	defer insertStmt.Close()

	if err := insertStmt.BindInt(1, embeddingsID); err != nil {
		return errors.WithStack(err)
	}

	if err := insertStmt.BindText(2, string(collectionID)); err != nil {
		return errors.WithStack(err)
	}

	if err := insertStmt.Exec(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Search implements port.Index.
func (i *Index) Search(ctx context.Context, query string, opts port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	var searchResults []*port.IndexSearchResult
	err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		res, err := i.llm.Embeddings(ctx, query)
		if err != nil {
			return errors.WithStack(err)
		}

		sql := `
		SELECT
			source,
			section_id,
			vec_distance_L2(embeddings, vec_normalize(vec_slice(?, 0, 256))) as distance
		FROM embeddings
	`

		if len(opts.Collections) > 0 {
			sql += ` LEFT JOIN embeddings_collections ON id = embeddings_id WHERE embeddings_collections.collection_id IN ( SELECT value FROM json_each(?) )`
		}

		sql += ` ORDER BY distance ASC`

		if opts.MaxResults > 0 {
			sql += ` LIMIT ?`
		}

		sql += `;`

		stmt, _, err := conn.Prepare(sql)
		if err != nil {
			return errors.WithStack(err)
		}

		defer stmt.Close()

		embeddings, err := sqlite_vec.SerializeFloat32(toFloat32(res.Embeddings()[0]))
		if err != nil {
			return errors.WithStack(err)
		}

		bindIndex := 1

		if err := stmt.BindBlob(bindIndex, embeddings); err != nil {
			return errors.WithStack(err)
		}

		bindIndex = 2

		if len(opts.Collections) > 0 {
			jsonCollections, err := json.Marshal(opts.Collections)
			if err != nil {
				return errors.WithStack(err)
			}

			if err := stmt.BindBlob(bindIndex, jsonCollections); err != nil {
				return errors.WithStack(err)
			}

			bindIndex = 3
		}

		if opts.MaxResults > 0 {
			if err := stmt.BindInt(bindIndex, opts.MaxResults); err != nil {
				return errors.WithStack(err)
			}
		}

		if err := stmt.Exec(); err != nil {
			return errors.WithStack(err)
		}

		mappedScores := map[string]float64{}
		mappedSections := map[string][]model.SectionID{}

		for stmt.Step() {
			source := stmt.ColumnText(0)
			sectionID := stmt.ColumnText(1)
			distance := stmt.ColumnFloat(2)

			if _, exists := mappedSections[source]; !exists {
				mappedSections[source] = make([]model.SectionID, 0)
			}

			if distance == 0 {
				distance = math.SmallestNonzeroFloat64
			}

			mappedSections[source] = append(mappedSections[source], model.SectionID(sectionID))
			mappedScores[source] += 1 / distance
		}

		if err := stmt.Err(); err != nil {
			return errors.WithStack(err)
		}

		searchResults = make([]*port.IndexSearchResult, 0)

		for rawSource, sectionIDs := range mappedSections {
			source, err := url.Parse(rawSource)
			if err != nil {
				return errors.WithStack(err)
			}

			searchResults = append(searchResults, &port.IndexSearchResult{
				Source:   source,
				Sections: sectionIDs,
			})
		}

		slices.SortFunc(searchResults, func(r1 *port.IndexSearchResult, r2 *port.IndexSearchResult) int {
			score1 := mappedScores[r1.Source.String()]
			score2 := mappedScores[r2.Source.String()]
			if score1 > score2 {
				return -1
			}
			if score1 < score2 {
				return 1
			}
			return 0
		})

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return searchResults, nil
}

func (i *Index) withRetry(ctx context.Context, fn func(ctx context.Context, conn *sqlite3.Conn) error, codes ...sqlite3.ErrorCode) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	conn, err := i.getConn(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	backoff := 500 * time.Millisecond
	maxRetries := 10
	retries := 0

	execWithSavepoint := func() (err error) {
		save := conn.Savepoint()
		defer save.Release(&err)

		if err = fn(ctx, conn); err != nil {
			err = errors.WithStack(err)
			return
		}

		err = nil
		return
	}

	for {
		if err := execWithSavepoint(); err != nil {
			slog.DebugContext(ctx, "transaction failed", slog.Any("error", errors.WithStack(err)))

			if retries >= maxRetries {
				return errors.WithStack(err)
			}

			var sqliteErr *sqlite3.Error
			if errors.As(err, &sqliteErr) {
				if !slices.Contains(codes, sqliteErr.Code()) {
					return errors.WithStack(err)
				}

				slog.DebugContext(ctx, "will retry transaction", slog.Int("retries", retries), slog.Duration("backoff", backoff))

				retries++
				time.Sleep(backoff)
				backoff *= 2
				continue
			}

			return errors.WithStack(err)
		}

		return nil
	}
}

func NewIndex(conn *sqlite3.Conn, llm llm.Client, model string, maxWords int) *Index {
	return &Index{
		maxWords: maxWords,
		llm:      llm,
		getConn:  createGetConn(conn),
	}
}

var _ port.Index = &Index{}

func createGetConn(conn *sqlite3.Conn) func(ctx context.Context) (*sqlite3.Conn, error) {
	var (
		migrateOnce sync.Once
		migrateErr  error
	)

	return func(ctx context.Context) (*sqlite3.Conn, error) {
		migrateOnce.Do(func() {
			if err := conn.Exec("PRAGMA journal_mode=wal; PRAGMA foreign_keys=on; PRAGMA busy_timeout=30000"); err != nil {
				migrateErr = errors.WithStack(err)
				return
			}

			for _, sql := range migrations {
				if err := conn.Exec(sql); err != nil {
					migrateErr = errors.Wrapf(err, "could not execute migration '%s'", sql)
					return
				}
			}
		})
		if migrateErr != nil {
			return nil, errors.WithStack(migrateErr)
		}

		return conn, nil
	}
}

func toFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}
