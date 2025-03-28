package sqlitevec

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/url"
	"slices"
	"sync"
	"unicode"

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
}

// DeleteBySource implements port.Index.
func (i *Index) DeleteBySource(ctx context.Context, source *url.URL) error {
	conn, err := i.getConn(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

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

	if err := stmt.Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Index implements port.Index.
func (i *Index) Index(ctx context.Context, document model.Document) error {
	source := document.Source()

	if err := i.DeleteBySource(ctx, source); err != nil {
		return errors.WithStack(err)
	}

	for _, s := range document.Sections() {
		if err := i.indexSection(ctx, s); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (i *Index) indexSection(ctx context.Context, section model.Section) (err error) {
	truncated := i.truncate(ctx, section.Content())

	res, err := i.llm.Embeddings(ctx, llm.WithInput(truncated))
	if err != nil {
		return errors.WithStack(err)
	}

	conn, err := i.getConn(ctx)
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

	if stmt.Step() {
		if err := stmt.Err(); err != nil {
			return errors.WithStack(err)
		}
	}

	embeddingsID := stmt.ColumnInt(0)

	stmt.Close()

	for _, coll := range section.Document().Collections() {
		if err := i.insertCollection(ctx, conn, int(embeddingsID), coll.ID()); err != nil {
			return errors.WithStack(err)
		}
	}

	for _, s := range section.Sections() {
		if err := i.indexSection(ctx, s); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (i *Index) insertCollection(ctx context.Context, conn *sqlite3.Conn, embeddingsID int, collectionID model.CollectionID) error {
	stmt, _, err := conn.Prepare("INSERT INTO embeddings_collections ( embeddings_id, collection_id ) VALUES (?, ?);")
	if err != nil {
		return errors.WithStack(err)
	}

	defer stmt.Close()

	if err := stmt.BindInt(1, embeddingsID); err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.BindText(2, string(collectionID)); err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.Exec(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (i *Index) truncate(ctx context.Context, text string) string {
	words := splitByWords(text)

	slog.DebugContext(ctx, "text size before truncate", slog.Int("textLength", len(text)), slog.Int("totalWords", len(words)))

	totalWords := len(words)

	if len(words) <= i.maxWords {
		return text
	}

	// Use middle-out strategy
	// See https://openrouter.ai/docs/features/message-transforms
	// and https://arxiv.org/abs/2307.03172

	halvedDiff := (totalWords - i.maxWords) / 2
	middle := totalWords / 2

	strippingStart := words[middle-halvedDiff].Start
	strippingEnd := words[middle+halvedDiff].End

	truncated := text[:strippingStart] + text[strippingEnd:]

	slog.DebugContext(ctx, "text size after truncate", slog.Int("textLength", len(truncated)), slog.Int("totalWords", len(words)-i.maxWords))

	return truncated
}

type Word struct {
	Start int
	End   int
}

func splitByWords(text string) []*Word {
	words := make([]*Word, 0)

	var word *Word
	for idx, rune := range text {
		if unicode.IsSpace(rune) || unicode.IsPunct(rune) {
			if word != nil {
				word.End = idx - 1
				words = append(words, word)
				word = nil
			}
		} else if word == nil {
			word = &Word{
				Start: idx,
			}
		}
	}

	if word != nil {
		word.End = len(text) - 1
		words = append(words, word)
	}

	return words
}

// Search implements port.Index.
func (i *Index) Search(ctx context.Context, query string, opts port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	conn, err := i.getConn(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	res, err := i.llm.Embeddings(ctx, llm.WithInput(query))
	if err != nil {
		return nil, errors.WithStack(err)
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

	slog.DebugContext(ctx, "executing vector index query", slog.String("query", sql))

	stmt, _, err := conn.Prepare(sql)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer stmt.Close()

	embeddings, err := sqlite_vec.SerializeFloat32(toFloat32(res.Embeddings()[0]))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	bindIndex := 1

	if err := stmt.BindBlob(bindIndex, embeddings); err != nil {
		return nil, errors.WithStack(err)
	}

	bindIndex = 2

	if len(opts.Collections) > 0 {
		jsonCollections, err := json.Marshal(opts.Collections)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if err := stmt.BindBlob(bindIndex, jsonCollections); err != nil {
			return nil, errors.WithStack(err)
		}

		bindIndex = 3
	}

	if opts.MaxResults > 0 {
		if err := stmt.BindInt(bindIndex, opts.MaxResults); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if err := stmt.Exec(); err != nil {
		return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}

	searchResults := make([]*port.IndexSearchResult, 0)

	for rawSource, sectionIDs := range mappedSections {
		source, err := url.Parse(rawSource)
		if err != nil {
			return nil, errors.WithStack(err)
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

	return searchResults, nil
}

func NewIndex(conn *sqlite3.Conn, llm llm.Client, maxWords int) *Index {
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
