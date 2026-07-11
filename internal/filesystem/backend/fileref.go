package backend

import (
	"encoding/base64"
	"os"

	"github.com/pkg/errors"
)

// FileRef references a file either by its base64-encoded content (for DB storage and UI uploads)
// or by a local filesystem path (for CLI usage only — never persisted to DB).
type FileRef struct {
	Content string `json:"content,omitempty" jsonschema:"description=Base64-encoded file content"`
	Path    string `json:"path,omitempty"    jsonschema:"description=Local filesystem path (CLI only)"`
}

// Read returns the raw file bytes, decoding from base64 or reading from disk as appropriate.
func (f *FileRef) Read() ([]byte, error) {
	if f.Content != "" {
		data, err := base64.StdEncoding.DecodeString(f.Content)
		if err != nil {
			return nil, errors.Wrap(err, "could not decode base64 file content")
		}
		return data, nil
	}
	if f.Path != "" {
		data, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "could not read file '%s'", f.Path)
		}
		return data, nil
	}
	return nil, errors.New("fileref: neither content nor path is set")
}

// FileRefFromBytes creates a FileRef with the given bytes encoded as base64.
func FileRefFromBytes(data []byte) FileRef {
	return FileRef{Content: base64.StdEncoding.EncodeToString(data)}
}
