package sqlitevec

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"io"

	"github.com/bornholm/corpus/internal/backup"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
)

func init() {
	gob.Register(SnapshottedRecord{})
	gob.Register(SnapshottedMetadata{})
}

type SnapshottedMetadata struct {
	Model string
}

type SnapshottedRecord struct {
	Source      string
	SectionID   string
	Embeddings  []byte
	Collections []string
}

// GenerateSnapshot implements snapshot.Snapshotable.
func (i *Index) GenerateSnapshot(ctx context.Context) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go func() {
		defer w.Close()

		encoder := gob.NewEncoder(w)

		metadata := SnapshottedMetadata{
			Model: i.model,
		}

		if err := encoder.Encode(metadata); err != nil {
			w.CloseWithError(errors.WithStack(err))
			return
		}

		err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
			sql := `
				SELECT
					e.id,
					e.source,
					e.section_id,
					e.embeddings,
					COALESCE(json_group_array(ec.collection_id), '[]') AS collections
				FROM embeddings e
				LEFT JOIN embeddings_collections ec ON e.id = ec.embeddings_id
				GROUP BY e.id, e.source, e.section_id
				;
			`

			stmt, _, err := conn.Prepare(sql)
			if err != nil {
				return errors.WithStack(err)
			}

			defer stmt.Close()

			for stmt.Step() {
				record := SnapshottedRecord{}
				record.Source = stmt.ColumnText(1)
				record.SectionID = stmt.ColumnText(2)
				record.Embeddings = stmt.ColumnBlob(3, []byte{})
				rawCollections := stmt.ColumnBlob(4, []byte{})
				if err := json.Unmarshal(rawCollections, &record.Collections); err != nil {
					return errors.WithStack(err)
				}

				if err := encoder.Encode(record); err != nil {
					return errors.WithStack(err)
				}
			}

			if err := stmt.Err(); err != nil {
				return errors.WithStack(err)
			}

			return nil
		}, sqlite3.LOCKED, sqlite3.BUSY)
		if err != nil {
			w.CloseWithError(errors.WithStack(err))
			return
		}
	}()

	return io.NopCloser(r), nil
}

// RestoreSnapshot implements snapshot.Snapshotable.
func (i *Index) RestoreSnapshot(ctx context.Context, r io.Reader) error {
	decoder := gob.NewDecoder(r)

	metadata := SnapshottedMetadata{}

	if err := decoder.Decode(&metadata); err != nil {
		return errors.WithStack(err)
	}

	if metadata.Model != i.model {
		return errors.Errorf("could not restore snapshot with a different embedding model '%s'", metadata.Model)
	}

	err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		if err := conn.Exec("DELETE FROM embeddings"); err != nil {
			return errors.WithStack(err)
		}

		for {
			var record SnapshottedRecord
			if err := decoder.Decode(&record); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}

				return errors.WithStack(err)
			}

			if err := i.restoreRecord(ctx, conn, record); err != nil {
				return errors.WithStack(err)
			}
		}
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (i *Index) restoreRecord(ctx context.Context, conn *sqlite3.Conn, record SnapshottedRecord) error {
	stmt, _, err := conn.Prepare("INSERT INTO embeddings ( source, section_id, embeddings ) VALUES (?, ?, ?) RETURNING id;")
	if err != nil {
		return errors.WithStack(err)
	}

	defer stmt.Close()

	if err := stmt.BindText(1, record.Source); err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.BindText(2, record.SectionID); err != nil {
		return errors.WithStack(err)
	}

	if err := stmt.BindBlob(3, record.Embeddings); err != nil {
		return errors.WithStack(err)
	}

	stmt.Step()

	if err := stmt.Err(); err != nil {
		return errors.WithStack(err)
	}

	embeddingsID := stmt.ColumnInt(0)

	for _, collectionID := range record.Collections {
		if err := i.insertCollection(ctx, conn, embeddingsID, model.CollectionID(collectionID)); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

var _ backup.Snapshotable = &Index{}
