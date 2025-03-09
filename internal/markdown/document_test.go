package markdown

import (
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
)

func TestBuildDocumentFrom(t *testing.T) {
	data, err := os.ReadFile("testdata/test.md")
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	t.Logf(spew.Sdump(doc))
	// t.Logf("[MARKDOWN]\n%s", doc.Root().Content())
}
