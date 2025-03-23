package bleve

import (
	"os"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/port/testsuite"
	_ "github.com/bornholm/genai/llm/provider/openai"
	"github.com/pkg/errors"
)

func TestIndex(t *testing.T) {
	testsuite.TestIndex(t, func(t *testing.T) (port.Index, error) {
		dataDir := "./testdata/index.bleve"

		if err := os.RemoveAll(dataDir); err != nil {
			return nil, errors.WithStack(err)
		}

		mapping := IndexMapping()

		bleveIndex, err := bleve.New(dataDir, mapping)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		index := NewIndex(bleveIndex)

		return index, nil
	})
}
