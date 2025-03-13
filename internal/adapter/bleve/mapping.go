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
	resourceMapping.AddFieldMappingsAt("content", contentFieldMapping)

	sourceFieldMapping := bleve.NewTextFieldMapping()
	sourceFieldMapping.Analyzer = AnalyzerDynamicLang
	sourceFieldMapping.Store = false
	resourceMapping.AddFieldMappingsAt("source", sourceFieldMapping)

	collectionFieldMapping := bleve.NewTextFieldMapping()
	collectionFieldMapping.Analyzer = AnalyzerDynamicLang
	collectionFieldMapping.Store = false
	resourceMapping.AddFieldMappingsAt("collection", collectionFieldMapping)

	mapping.AddDocumentMapping("resource", resourceMapping)

	return mapping
}
