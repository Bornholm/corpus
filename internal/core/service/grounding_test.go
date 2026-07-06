package service

import (
	"context"
	"net/url"
	"testing"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/genai/llm"
)

// --- test doubles ---------------------------------------------------------

// stubLLM implements llm.Client by embedding the interface (so only the methods
// exercised by the tests need real bodies) and returns a canned completion.
type stubLLM struct {
	llm.Client
	response string
	calls    int
}

func (m *stubLLM) ChatCompletion(ctx context.Context, funcs ...llm.ChatCompletionOptionFunc) (llm.ChatCompletionResponse, error) {
	m.calls++
	return llm.NewChatCompletionResponse(
		llm.NewMessage(llm.RoleAssistant, m.response),
		llm.NewChatCompletionUsage(0, 0, 0),
	), nil
}

// stubSection implements model.Section by embedding the interface.
type stubSection struct {
	model.Section
	id      model.SectionID
	content string
}

func (s *stubSection) ID() model.SectionID      { return s.id }
func (s *stubSection) Content() ([]byte, error) { return []byte(s.content), nil }

// stubStore implements port.DocumentStore by embedding the interface; only the
// two section accessors used by the grounding checker / generateResponse are
// implemented.
type stubStore struct {
	port.DocumentStore
	sections map[model.SectionID]model.Section
}

func (s *stubStore) GetSectionByID(ctx context.Context, id model.SectionID) (model.Section, error) {
	return s.sections[id], nil
}

func (s *stubStore) GetSectionsByIDs(ctx context.Context, ids []model.SectionID) (map[model.SectionID]model.Section, error) {
	out := map[model.SectionID]model.Section{}
	for _, id := range ids {
		if sec, ok := s.sections[id]; ok {
			out[id] = sec
		}
	}
	return out, nil
}

// fakeChecker is a GroundingChecker returning a fixed verdict.
type fakeChecker struct {
	result *GroundingResult
	calls  int
}

func (f *fakeChecker) Check(ctx context.Context, query string, results []*port.IndexSearchResult) (*GroundingResult, error) {
	f.calls++
	return f.result, nil
}

func newStubStore() *stubStore {
	return &stubStore{
		sections: map[model.SectionID]model.Section{
			"sec-1": &stubSection{id: "sec-1", content: "Paris is the capital of France."},
		},
	}
}

func oneResult() []*port.IndexSearchResult {
	return []*port.IndexSearchResult{
		{Source: &url.URL{Scheme: "test", Host: "doc"}, Sections: []model.SectionID{"sec-1"}},
	}
}

// --- LLMGroundingChecker --------------------------------------------------

func TestLLMGroundingChecker_EmptyResults(t *testing.T) {
	checker := NewLLMGroundingChecker(&stubLLM{}, newStubStore(), 0)

	got, err := checker.Check(context.Background(), "q", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != GroundingInvalid {
		t.Fatalf("expected invalid on empty results, got %q", got.Status)
	}
}

func TestLLMGroundingChecker_ParsesVerdict(t *testing.T) {
	llmClient := &stubLLM{response: `{"status":"valid","score":0.9,"explanation":"fully supported"}`}
	checker := NewLLMGroundingChecker(llmClient, newStubStore(), 0)

	got, err := checker.Check(context.Background(), "In which country is Paris?", oneResult())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != GroundingValid {
		t.Fatalf("expected valid, got %q", got.Status)
	}
	if got.Score != 0.9 {
		t.Fatalf("expected score 0.9, got %v", got.Score)
	}
	if llmClient.calls != 1 {
		t.Fatalf("expected exactly 1 LLM call, got %d", llmClient.calls)
	}
}

func TestLLMGroundingChecker_ClampsScoreAndNormalizesStatus(t *testing.T) {
	llmClient := &stubLLM{response: `{"status":"WeIrD","score":1.7,"explanation":""}`}
	checker := NewLLMGroundingChecker(llmClient, newStubStore(), 0)

	got, err := checker.Check(context.Background(), "q", oneResult())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != GroundingInvalid {
		t.Fatalf("unknown status should normalize to invalid, got %q", got.Status)
	}
	if got.Score != 1.0 {
		t.Fatalf("score should clamp to 1.0, got %v", got.Score)
	}
}

// --- DocumentManager.Ask grounding gate -----------------------------------

func TestAsk_AbstainsWhenInvalid(t *testing.T) {
	checker := &fakeChecker{result: &GroundingResult{Status: GroundingInvalid, Score: 0.1, Explanation: "off-topic"}}
	llmClient := &stubLLM{response: "should not be produced"}

	dm := NewDocumentManager(newStubStore(), nil, nil, llmClient,
		WithGroundingChecker(checker),
	)

	var grounding GroundingResult
	answer, contents, err := dm.Ask(context.Background(), "q", oneResult(),
		WithAskGroundingOutput(&grounding),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llmClient.calls != 0 {
		t.Fatalf("generation LLM must not be called on abstention, got %d calls", llmClient.calls)
	}
	if len(contents) != 0 {
		t.Fatalf("expected empty contents on abstention, got %d", len(contents))
	}
	if grounding.Status != GroundingInvalid {
		t.Fatalf("grounding output not populated, got %q", grounding.Status)
	}
	if answer == "" || answer == "should not be produced" {
		t.Fatalf("expected an abstention message, got %q", answer)
	}
}

func TestAsk_AnswersWhenValid(t *testing.T) {
	checker := &fakeChecker{result: &GroundingResult{Status: GroundingValid, Score: 0.9}}
	llmClient := &stubLLM{response: "France"}

	dm := NewDocumentManager(newStubStore(), nil, nil, llmClient,
		WithGroundingChecker(checker),
	)

	answer, contents, err := dm.Ask(context.Background(), "In which country is Paris?", oneResult())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "France" {
		t.Fatalf("expected generated answer, got %q", answer)
	}
	if len(contents) != 1 {
		t.Fatalf("expected contents populated with 1 section, got %d", len(contents))
	}
	if checker.calls != 1 {
		t.Fatalf("expected grounding checked once, got %d", checker.calls)
	}
}
