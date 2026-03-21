package bleve

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

func IndexMapping() *mapping.IndexMappingImpl {
	mapping := bleve.NewIndexMapping()

	mapping.TypeField = "_type"
	mapping.DefaultAnalyzer = AnalyzerDynamicLang

	resourceMapping := bleve.NewDocumentMapping()

	contentFieldMapping := bleve.NewTextFieldMapping()
	contentFieldMapping.Analyzer = AnalyzerDynamicLang
	contentFieldMapping.Store = false
	contentFieldMapping.IncludeTermVectors = true
	resourceMapping.AddFieldMappingsAt("content", contentFieldMapping)

	sourceFieldMapping := bleve.NewTextFieldMapping()
	sourceFieldMapping.Analyzer = AnalyzerDynamicLang
	sourceFieldMapping.Store = true
	sourceFieldMapping.IncludeTermVectors = true
	resourceMapping.AddFieldMappingsAt("source", sourceFieldMapping)

	collectionsFieldMapping := bleve.NewTextFieldMapping()
	collectionsFieldMapping.Analyzer = AnalyzerDynamicLang
	collectionsFieldMapping.Store = false
	collectionsFieldMapping.IncludeTermVectors = true
	resourceMapping.AddFieldMappingsAt("collections", collectionsFieldMapping)

	mapping.AddDocumentMapping("resource", resourceMapping)

	return mapping
}
