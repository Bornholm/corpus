package sqlitevec

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/ncruces/go-sqlite3"
	"github.com/pkg/errors"
)

// All implements port.Index.
func (i *Index) All(ctx context.Context, yield func(model.SectionID) bool) error {
	err := i.withRetry(ctx, func(ctx context.Context, conn *sqlite3.Conn) error {
		sql := `SELECT section_id FROM embeddings;`

		stmt, _, err := conn.Prepare(sql)
		if err != nil {
			return errors.WithStack(err)
		}

		defer stmt.Close()

		for stmt.Step() {
			sectionID := model.SectionID(stmt.ColumnText(0))
			if !yield(sectionID) {
				return nil
			}
		}

		if err := stmt.Err(); err != nil {
			return errors.WithStack(err)
		}

		return nil
	}, sqlite3.LOCKED, sqlite3.BUSY)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
