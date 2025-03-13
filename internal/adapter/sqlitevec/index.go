package sqlitevec

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"sync"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/genai/llm"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

type Index struct {
	getConn func(ctx context.Context) (*sqlite3.Conn, error)
	llm     llm.Client
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

func (i *Index) indexSection(ctx context.Context, section model.Section) error {
	res, err := i.llm.Embeddings(ctx, llm.WithInput(section.Content()))
	if err != nil {
		return errors.WithStack(err)
	}

	conn, err := i.getConn(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	stmt, _, err := conn.Prepare("INSERT INTO embeddings (source, section_id, embeddings, collection) VALUES (?, ?, ?, ?);")
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

	if err := stmt.BindText(4, string(section.Document().Collection())); err != nil {
		return errors.WithStack(err)
	}

	slog.DebugContext(ctx, "indexing section", slog.Any("source", section.Document().Source()), slog.Any("sectionID", section.ID()))

	if err := stmt.Exec(); err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.Err(); err != nil {
		return errors.WithStack(err)
	}

	stmt.Close()

	slog.DebugContext(ctx, "section indexed", slog.Any("source", section.Document().Source()), slog.Any("sectionID", section.ID()))

	for _, s := range section.Sections() {
		if err := i.indexSection(ctx, s); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

const hydePromptTemplate = `
As a knowledgeable and helpful research assistant, your task is to provide informative informations about the given context. Use your extensive knowledge base to offer clear, concise, and accurate responses to the user's inquiries.

Do not output anything than your answer.

Topic: {{ .Query }}
`

// Search implements port.Index.
func (i *Index) Search(ctx context.Context, query string, opts *port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	conn, err := i.getConn(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	prompt, err := llm.PromptTemplate(hydePromptTemplate, struct {
		Query string
	}{
		Query: query,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	completion, err := i.llm.ChatCompletion(ctx,
		llm.WithMessages(
			llm.NewMessage(llm.RoleUser, prompt),
		),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	answer := completion.Message().Content()

	slog.DebugContext(ctx, "generated hypothetic answer", slog.String("answer", answer))

	res, err := i.llm.Embeddings(ctx, llm.WithInput(answer))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sql := `
		SELECT
			source,
			section_id,
			vec_distance_L2(embeddings, ?) as distance
		FROM embeddings
		WHERE 1 = 1
	`

	if opts != nil && opts.Collections != nil {
		sql += ` AND collection IN ( 
			SELECT value FROM json_each( ? )
		)`
	}

	sql += ` ORDER BY distance`

	if opts != nil && opts.MaxResults > 0 {
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

	if opts != nil && opts.Collections != nil {
		jsonCollections, err := json.Marshal(opts.Collections)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if err := stmt.BindBlob(bindIndex, jsonCollections); err != nil {
			return nil, errors.WithStack(err)
		}

		bindIndex = 3
	}

	if opts != nil && opts.MaxResults > 0 {
		if err := stmt.BindInt(bindIndex, opts.MaxResults); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	if err := stmt.Exec(); err != nil {
		return nil, errors.WithStack(err)
	}

	mappedSections := map[string][]model.SectionID{}

	for stmt.Step() {
		source := stmt.ColumnText(0)
		sectionID := stmt.ColumnText(1)

		if _, exists := mappedSections[source]; !exists {
			mappedSections[source] = make([]model.SectionID, 0)
		}

		mappedSections[source] = append(mappedSections[source], model.SectionID(sectionID))
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

	return searchResults, nil
}

func NewIndex(conn *sqlite3.Conn, llm llm.Client) *Index {
	return &Index{
		llm:     llm,
		getConn: createGetConn(conn),
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
