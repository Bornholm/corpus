package service

import (
	"context"
	"net/url"
	"testing"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
)

// --- extra test doubles ---------------------------------------------------

// stubIndex implements port.Index, returning canned results per query and
// recording the queries it was asked. port.Index has a method named Index, so
// the interface cannot be embedded (field/method name clash); the unused methods
// are implemented explicitly.
type stubIndex struct {
	byQuery map[string][]*port.IndexSearchResult
	calls   []string
}

func (i *stubIndex) Search(ctx context.Context, query string, opts port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	i.calls = append(i.calls, query)
	return i.byQuery[query], nil
}

func (i *stubIndex) Index(ctx context.Context, document model.Document, funcs ...port.IndexOptionFunc) error {
	return nil
}
func (i *stubIndex) DeleteBySource(ctx context.Context, source *url.URL) error       { return nil }
func (i *stubIndex) DeleteByID(ctx context.Context, ids ...model.SectionID) error    { return nil }
func (i *stubIndex) All(ctx context.Context, yield func(model.SectionID) bool) error { return nil }

var _ port.Index = &stubIndex{}

// scriptedChecker returns a scripted sequence of verdicts (last one repeats).
type scriptedChecker struct {
	verdicts []*GroundingResult
	calls    int
}

func (c *scriptedChecker) Check(ctx context.Context, query string, results []*port.IndexSearchResult) (*GroundingResult, error) {
	idx := c.calls
	if idx >= len(c.verdicts) {
		idx = len(c.verdicts) - 1
	}
	c.calls++
	return c.verdicts[idx], nil
}

type fakeReformulator struct {
	out   string
	calls int
}

func (f *fakeReformulator) Reformulate(ctx context.Context, query string, hint string) (string, error) {
	f.calls++
	return f.out, nil
}

type fakeDecomposer struct {
	subs  []string
	calls int
}

func (f *fakeDecomposer) Decompose(ctx context.Context, query string) ([]string, error) {
	f.calls++
	return f.subs, nil
}

func resultWith(sectionIDs ...model.SectionID) *port.IndexSearchResult {
	return &port.IndexSearchResult{
		Source:   &url.URL{Scheme: "test", Host: "doc"},
		Sections: sectionIDs,
	}
}

func storeWithSections(ids ...model.SectionID) *stubStore {
	sections := map[model.SectionID]model.Section{}
	for _, id := range ids {
		sections[id] = &stubSection{id: id, content: "content of " + string(id)}
	}
	return &stubStore{sections: sections}
}

// --- fuseResults ----------------------------------------------------------

func TestFuseResults_DedupsSectionsAndDropsEmpty(t *testing.T) {
	a := []*port.IndexSearchResult{resultWith("s1", "s2")}
	b := []*port.IndexSearchResult{resultWith("s2", "s3"), resultWith("s1")}

	fused := fuseResults(a, b)

	// s1,s2 from a; s3 from b; the second b-result (only s1) is fully duplicate → dropped.
	got := map[model.SectionID]int{}
	for _, r := range fused {
		for _, s := range r.Sections {
			got[s]++
		}
	}

	for _, s := range []model.SectionID{"s1", "s2", "s3"} {
		if got[s] != 1 {
			t.Fatalf("section %q should appear exactly once, got %d", s, got[s])
		}
	}
	if len(fused) != 2 {
		t.Fatalf("expected 2 non-empty results, got %d", len(fused))
	}
}

// --- iterative re-retrieval (Phase 2) -------------------------------------

func TestAskWithRetrieval_IterativeReRetrieval(t *testing.T) {
	index := &stubIndex{byQuery: map[string][]*port.IndexSearchResult{
		"q":            {resultWith("sec-1")},
		"reformulated": {resultWith("sec-2")},
	}}
	checker := &scriptedChecker{verdicts: []*GroundingResult{
		{Status: GroundingInvalid, Score: 0.1},
		{Status: GroundingValid, Score: 0.9},
	}}
	reformulator := &fakeReformulator{out: "reformulated"}

	dm := NewDocumentManager(storeWithSections("sec-1", "sec-2"), index, nil, &stubLLM{response: "answer"},
		WithGroundingChecker(checker),
		WithQueryReformulator(reformulator),
		WithIterativeMaxRounds(1),
	)

	result, err := dm.AskWithRetrieval(context.Background(), "q", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Rounds != 1 {
		t.Fatalf("expected exactly 1 re-retrieval round, got %d", result.Rounds)
	}
	if reformulator.calls != 1 {
		t.Fatalf("expected reformulator called once, got %d", reformulator.calls)
	}
	if len(index.calls) != 2 || index.calls[0] != "q" || index.calls[1] != "reformulated" {
		t.Fatalf("expected searches [q reformulated], got %v", index.calls)
	}
	if result.Answer != "answer" {
		t.Fatalf("expected generated answer after successful re-retrieval, got %q", result.Answer)
	}
	if result.Grounding == nil || result.Grounding.Status != GroundingValid {
		t.Fatalf("expected final grounding valid, got %+v", result.Grounding)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected fused evidence of 2 results, got %d", len(result.Results))
	}
}

func TestAskWithRetrieval_AbstainsAfterMaxRounds(t *testing.T) {
	index := &stubIndex{byQuery: map[string][]*port.IndexSearchResult{
		"q":     {resultWith("sec-1")},
		"again": {resultWith("sec-2")},
	}}
	checker := &scriptedChecker{verdicts: []*GroundingResult{
		{Status: GroundingInvalid, Score: 0.1, Explanation: "missing"},
	}}
	reformulator := &fakeReformulator{out: "again"}

	dm := NewDocumentManager(storeWithSections("sec-1", "sec-2"), index, nil, &stubLLM{response: "should not generate"},
		WithGroundingChecker(checker),
		WithQueryReformulator(reformulator),
		WithIterativeMaxRounds(1),
	)

	result, err := dm.AskWithRetrieval(context.Background(), "q", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Rounds != 1 {
		t.Fatalf("expected 1 round before giving up, got %d", result.Rounds)
	}
	if len(result.Contents) != 0 {
		t.Fatalf("expected empty contents on abstention, got %d", len(result.Contents))
	}
	if result.Answer == "should not generate" || result.Answer == "" {
		t.Fatalf("expected an abstention message, got %q", result.Answer)
	}
}

// --- query decomposition (Phase 3) ----------------------------------------

func TestAskWithRetrieval_Decomposition(t *testing.T) {
	index := &stubIndex{byQuery: map[string][]*port.IndexSearchResult{
		"q":    {resultWith("sec-a")},
		"sub1": {resultWith("sec-b")},
		"sub2": {resultWith("sec-c")},
	}}
	decomposer := &fakeDecomposer{subs: []string{"sub1", "sub2"}}

	dm := NewDocumentManager(storeWithSections("sec-a", "sec-b", "sec-c"), index, nil, &stubLLM{response: "answer"},
		WithQueryDecomposer(decomposer),
	)

	result, err := dm.AskWithRetrieval(context.Background(), "q", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decomposer.calls != 1 {
		t.Fatalf("expected decomposer called once, got %d", decomposer.calls)
	}
	if len(index.calls) != 3 {
		t.Fatalf("expected 3 searches (original + 2 sub-questions), got %v", index.calls)
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected fused evidence of 3 results, got %d", len(result.Results))
	}
	if result.Answer != "answer" {
		t.Fatalf("expected generated answer, got %q", result.Answer)
	}
	if result.Grounding != nil {
		t.Fatalf("grounding should be nil when checker disabled, got %+v", result.Grounding)
	}
}
