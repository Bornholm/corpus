package markdown

import (
	"os"
	"testing"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/pkg/errors"
)

func TestBuildDocumentFrom(t *testing.T) {
	type testCase struct {
		File             string
		ExpectedSections int
	}

	testCases := []testCase{
		{
			File:             "testdata/test.md",
			ExpectedSections: 7,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.File, func(t *testing.T) {
			data, err := os.ReadFile(tc.File)
			if err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			doc, err := Parse(data)
			if err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			if e, g := tc.ExpectedSections, countSections(doc); e != g {
				t.Errorf("len(doc.Sections()): expected '%d', got '%v'", e, g)
			}
		})
	}
}

type Sections interface {
	Sections() []model.Section
}

func countSections(root Sections) int {
	total := 0
	for _, s := range root.Sections() {
		total += 1 + countSections(s)
	}
	return total
}
