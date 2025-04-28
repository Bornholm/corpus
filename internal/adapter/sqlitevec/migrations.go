package sqlitevec

var migrations = []string{
	"CREATE TABLE IF NOT EXISTS embeddings (id INTEGER NOT NULL PRIMARY KEY, source TEXT, section_id TEXT UNIQUE, embeddings FLOAT[256]);",
	"CREATE INDEX IF NOT EXISTS embeddings_source_idx ON embeddings ( source );",
	"CREATE TABLE IF NOT EXISTS embeddings_collections ( embeddings_id INTEGER, collection_id TEXT NOT NULL, FOREIGN KEY (embeddings_id) REFERENCES embeddings (id) ON DELETE CASCADE );",
	"CREATE INDEX IF NOT EXISTS embeddings_collections_idx ON embeddings_collections ( embeddings_id, collection_id );",
}
