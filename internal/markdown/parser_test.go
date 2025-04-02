package markdown

import (
	"os"
	"strings"
	"testing"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/text"
	"github.com/pkg/errors"
)

func TestParser(t *testing.T) {
	type testCase struct {
		File              string
		ExpectedSections  int
		MaxWordPerSection int
	}

	testCases := []testCase{
		{
			File:              "testdata/test.md",
			ExpectedSections:  7,
			MaxWordPerSection: 5,
		},
		{
			File:              "testdata/le_horla.md",
			ExpectedSections:  220,
			MaxWordPerSection: 200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.File, func(t *testing.T) {
			data, err := os.ReadFile(tc.File)
			if err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			opts := []OptionFunc{}
			if tc.MaxWordPerSection > 0 {
				opts = append(opts, WithMaxWordPerSection(tc.MaxWordPerSection))
			}

			doc, err := Parse(data, opts...)
			if err != nil {
				t.Fatalf("%+v", errors.WithStack(err))
			}

			if e, g := tc.ExpectedSections, countSections(doc); e != g {
				t.Errorf("len(doc.Sections()): expected '%d', got '%v'", e, g)
			}

			dumpDocument(t, doc)
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

func dumpDocument(t *testing.T, doc *Document) {
	t.Logf("Document #%s", doc.ID())
	t.Logf("├─ Total sections: %d", countSections(doc))
	t.Log("├─ Sections")
	for _, s := range doc.Sections() {
		dumpSection(t, s, "  ")
	}
	t.Log("---")
}

func dumpSection(t *testing.T, section model.Section, indent string) {
	content := section.Content()
	t.Logf("%s│", indent)
	t.Logf("%s├─ #%s (level: %v, characters: %d, words: %d)", indent, section.ID(), section.Level(), len(content), len(text.SplitByWords(content)))
	t.Logf("%s├─ Branch: %v", indent, section.Branch())
	t.Logf("%s│", indent)
	t.Logf("%s│%s", indent, strings.ReplaceAll(text.MiddleOut(content, 10, " [...] "), "\n", " "))
	t.Logf("%s│", indent)
	if len(section.Sections()) > 0 {
		t.Logf("%s├─ Sections", indent)
		for _, ss := range section.Sections() {
			dumpSection(t, ss, indent+"  ")
		}
	}
}
