package setup

import (
	"context"
	"os"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	bleveAdapter "github.com/bornholm/corpus/internal/adapter/bleve"
	"github.com/bornholm/corpus/internal/config"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

func NewBleveIndexFromConfig(ctx context.Context, conf *config.Config) (port.Index, error) {
	var (
		index bleve.Index
		err   error
	)

	stat, err := os.Stat(conf.Storage.Index.DSN)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.WithStack(err)
	}

	if stat == nil {
		mapping := createIndexMapping()

		index, err = bleve.New(conf.Storage.Index.DSN, mapping)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	} else {
		index, err = bleve.Open(conf.Storage.Index.DSN)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return bleveAdapter.NewIndex(index), nil
}

func createIndexMapping() *mapping.IndexMappingImpl {
	mapping := bleve.NewIndexMapping()

	mapping.TypeField = "_type"
	mapping.DefaultAnalyzer = bleveAdapter.AnalyzerDynamicLang

	resourceMapping := bleve.NewDocumentMapping()

	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = bleveAdapter.AnalyzerDynamicLang
	resourceMapping.AddFieldMappingsAt("content", contentFieldMapping)

	sourceFieldMapping := bleve.NewTextFieldMapping()
	sourceFieldMapping.Analyzer = bleveAdapter.AnalyzerDynamicLang
	resourceMapping.AddFieldMappingsAt("source", sourceFieldMapping)

	mapping.AddDocumentMapping("resource", resourceMapping)

	return mapping
}
