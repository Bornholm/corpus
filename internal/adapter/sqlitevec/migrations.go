package sqlitevec

var migrations = []string{
	"CREATE TABLE IF NOT EXISTS embeddings (source TEXT, section_id TEXT, collection TEXT, embeddings FLOAT[1024]);",
	"CREATE INDEX IF NOT EXISTS embeddings_collection_idx ON embeddings ( collection );",
}
