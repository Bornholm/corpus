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
	// rwLock allows concurrent Search operations while serializing Index/Delete
	rwLock sync.RWMutex
}

// DeleteByID implements port.Index.
func (i *Index) DeleteByID(ctx context.Context, ids ...model.SectionID) error {
	i.rwLock.Lock()
	defer i.rwLock.Unlock()

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
	i.rwLock.Lock()
	defer i.rwLock.Unlock()

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

type indexableChunk struct {
	Section  model.Section
	Text     string
	ChunkIdx int
}

func estimateTokens(text string) int {
	return len(text) / charsPerToken
}

const (
	charsPerToken = 4

	maxBatchItemCount = 100

	targetBatchTokens = 6000

	overlapChars = 200
)

// Index implements port.Index.
func (i *Index) Index(ctx context.Context, document model.Document, funcs ...port.IndexOptionFunc) error {
	i.rwLock.Lock()
	defer i.rwLock.Unlock()

	opts := port.NewIndexOptions(funcs...)

	var chunksToProcess []*indexableChunk

	limitChars := i.maxWords * 6

	var collect func(s model.Section) error
	collect = func(s model.Section) error {
		content, err := s.Content()
		if err != nil {
			return err
		}
		textStr := string(content)
		textLen := len(textStr)

		if textLen <= limitChars {
			if textLen > 0 { // On ignore les sections vides
				chunksToProcess = append(chunksToProcess, &indexableChunk{
					Section:  s,
					Text:     textStr,
					ChunkIdx: 0,
				})
			}
		} else {
			runes := []rune(textStr)
			runesLen := len(runes)

			limitRunes := limitChars
			overlapRunes := overlapChars

			currentChunkIdx := 0
			for start := 0; start < runesLen; {
				end := start + limitRunes
				if end > runesLen {
					end = runesLen
				}

				chunkText := string(runes[start:end])

				chunksToProcess = append(chunksToProcess, &indexableChunk{
					Section:  s,
					Text:     chunkText,
					ChunkIdx: currentChunkIdx,
				})
				currentChunkIdx++

				if end == runesLen {
					break
				}

				start += (limitRunes - overlapRunes)
			}
		}

		for _, child := range s.Sections() {
			if err := collect(child); err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	}

	for _, s := range document.Sections() {
		if err := collect(s); err != nil {
			return errors.WithStack(err)
		}
	}

	defer func() {
		if opts.OnProgress != nil {
			opts.OnProgress(1.0)
		}
	}()

	if len(chunksToProcess) == 0 {
		return nil
	}

	return i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		stmt, _, err := conn.Prepare(`
			INSERT INTO embeddings (source, section_id, chunk_index, embeddings) 
			VALUES (?, ?, ?, vec_normalize(vec_slice(?, 0, 256)))
			RETURNING id;
		`)
		if err != nil {
			return errors.WithStack(err)
		}
		defer stmt.Close()

		var batchItems []*indexableChunk
		var batchTexts []string
		currentBatchTokens := 0

		flushBatch := func() error {
			if len(batchItems) == 0 {
				return nil
			}

			res, err := i.llm.Embeddings(ctx, batchTexts)
			if err != nil {
				return errors.Wrap(err, "generation failed")
			}

			embeddings := res.Embeddings()

			if len(embeddings) != len(batchItems) {
				return errors.New("vector count mismatch")
			}

			for idx, item := range batchItems {
				vecBlob, err := sqlite_vec.SerializeFloat32(toFloat32(embeddings[idx]))
				if err != nil {
					return err
				}

				if err := stmt.BindText(1, item.Section.Document().Source().String()); err != nil {
					return err
				}
				if err := stmt.BindText(2, string(item.Section.ID())); err != nil {
					return err
				}
				if err := stmt.BindInt64(3, int64(item.ChunkIdx)); err != nil {
					return err
				}

				if err := stmt.BindBlob(4, vecBlob); err != nil {
					return err
				}

				if hasRow := stmt.Step(); !hasRow {
					return errors.New("no id returned")
				}

				embeddingsID := stmt.ColumnInt(0)
				stmt.Reset()

				for _, coll := range item.Section.Document().Collections() {
					if err := i.insertCollection(ctx, conn, embeddingsID, coll.ID()); err != nil {
						return errors.WithStack(err)
					}
				}
			}
			return nil
		}

		for _, chunk := range chunksToProcess {
			tokenEst := estimateTokens(chunk.Text)

			isBatchFull := (len(batchItems) >= maxBatchItemCount) ||
				(currentBatchTokens+tokenEst >= targetBatchTokens)

			if isBatchFull {
				if err := flushBatch(); err != nil {
					return err
				}
				batchItems = nil
				batchTexts = nil
				currentBatchTokens = 0
			}

			batchItems = append(batchItems, chunk)
			batchTexts = append(batchTexts, chunk.Text)
			currentBatchTokens += tokenEst
		}

		if len(batchItems) > 0 {
			if err := flushBatch(); err != nil {
				return err
			}
		}

		return nil
	}, sqlite3.BUSY, sqlite3.LOCKED)
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
	i.rwLock.RLock()
	defer i.rwLock.RUnlock()

	var searchResults []*port.IndexSearchResult
	err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		res, err := i.llm.Embeddings(ctx, []string{query})
		if err != nil {
			return errors.WithStack(err)
		}

		sql := `
		SELECT
			source,
			section_id,
			vec_distance_L2(vec_normalize(vec_slice(embeddings, 0, 256)), vec_normalize(vec_slice(?, 0, 256))) as distance
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
