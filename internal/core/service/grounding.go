package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/bornholm/corpus/internal/text"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/genai/llm"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

// GroundingStatus qualifies whether the retrieved evidence supports a reliable,
// grounded answer to a query. It is the discrete form of the "γ" signal ported
// from MothRAG.
type GroundingStatus string

const (
	GroundingValid   GroundingStatus = "valid"
	GroundingPartial GroundingStatus = "partial"
	GroundingInvalid GroundingStatus = "invalid"
)

// GroundingResult is the verdict returned by a GroundingChecker.
type GroundingResult struct {
	Status      GroundingStatus `json:"status"`
	Score       float64         `json:"score"`
	Explanation string          `json:"explanation"`
}

// GroundingChecker decides whether a set of retrieved results supports a
// grounded answer to the query. It is consumed by DocumentManager.Ask to drive
// abstention (and, later, iterative re-retrieval).
type GroundingChecker interface {
	Check(ctx context.Context, query string, results []*port.IndexSearchResult) (*GroundingResult, error)
}

const defaultGroundingPrompt = `
You are a grounding verifier for a retrieval-augmented answering system.

## Input
- **Query**: the user's question
- **Documents**: retrieved passages, each with an identifier and content

## Task
Decide whether the Documents contain enough information to answer the Query
**using only those Documents**, without any outside knowledge:
- "valid": the Documents fully support a direct, reliable answer.
- "partial": the Documents are related and partially support an answer, but key facts are missing.
- "invalid": the Documents do not support a reliable answer to the Query.

Also give a numeric "score" in [0,1]: your confidence that a reliable, grounded
answer to the Query can be produced from the Documents alone.

## Output Format (strict JSON, no markdown fencing)
{"status": "valid", "score": 0.0, "explanation": "Brief justification"}
`

// LLMGroundingChecker implements GroundingChecker with a single structured-JSON
// LLM call. It mirrors JudgeResultsTransformer's evidence assembly and JSON
// output conventions.
type LLMGroundingChecker struct {
	llm           llm.Client
	store         port.DocumentStore
	maxTotalWords int
}

// NewLLMGroundingChecker builds a GroundingChecker backed by the given LLM
// client. maxTotalWords bounds the evidence budget (defaults to 50000 when <= 0).
func NewLLMGroundingChecker(client llm.Client, store port.DocumentStore, maxTotalWords int) *LLMGroundingChecker {
	return &LLMGroundingChecker{
		llm:           client,
		store:         store,
		maxTotalWords: maxTotalWords,
	}
}

// Check implements GroundingChecker.
func (c *LLMGroundingChecker) Check(ctx context.Context, query string, results []*port.IndexSearchResult) (*GroundingResult, error) {
	// No evidence at all → nothing to ground on.
	if len(results) == 0 {
		return &GroundingResult{
			Status:      GroundingInvalid,
			Score:       0,
			Explanation: "no documents were retrieved",
		}, nil
	}

	userPrompt, err := c.getUserPrompt(ctx, query, results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	seed, err := text.IntHash(defaultGroundingPrompt + query)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx = slogx.WithAttrs(ctx, slog.Int("seed", seed))

	completion, err := c.llm.ChatCompletion(ctx,
		llm.WithJSONResponse(
			llm.NewResponseSchema(
				"Grounding",
				"Whether the retrieved documents support a reliable answer to the query",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"valid", "partial", "invalid"},
							"description": "Whether the documents support a reliable answer",
						},
						"score": map[string]any{
							"type":        "number",
							"description": "Confidence in [0,1] that a grounded answer can be produced",
						},
						"explanation": map[string]any{
							"type":        "string",
							"description": "Brief justification of the verdict",
						},
					},
					"required":             []string{"status", "score", "explanation"},
					"additionalProperties": false,
				},
			),
		),
		llm.WithMessages(
			llm.NewMessage(llm.RoleSystem, defaultGroundingPrompt),
			llm.NewMessage(llm.RoleUser, userPrompt),
		),
		llm.WithTemperature(0),
		llm.WithSeed(seed),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	type llmResponse struct {
		Status      string  `json:"status"`
		Score       float64 `json:"score"`
		Explanation string  `json:"explanation"`
	}

	responses, err := llm.ParseJSON[llmResponse](completion.Message())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Missing signal: fail open (do not block answering) with a neutral verdict.
	if len(responses) == 0 {
		return &GroundingResult{
			Status:      GroundingPartial,
			Score:       0.5,
			Explanation: "grounding verifier returned no verdict",
		}, nil
	}

	r := responses[0]

	score := r.Score
	if score < 0 {
		score = 0
	} else if score > 1 {
		score = 1
	}

	result := &GroundingResult{
		Status:      normalizeGroundingStatus(r.Status),
		Score:       score,
		Explanation: r.Explanation,
	}

	slog.DebugContext(ctx, "grounding verdict",
		slog.String("status", string(result.Status)),
		slog.Float64("score", result.Score),
	)

	return result, nil
}

func normalizeGroundingStatus(s string) GroundingStatus {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "valid":
		return GroundingValid
	case "partial":
		return GroundingPartial
	default:
		return GroundingInvalid
	}
}

func (c *LLMGroundingChecker) getUserPrompt(ctx context.Context, query string, results []*port.IndexSearchResult) (string, error) {
	// Collect all section IDs for batch loading.
	var allSectionIDs []model.SectionID
	for _, r := range results {
		allSectionIDs = append(allSectionIDs, r.Sections...)
	}

	sectionsMap, err := c.store.GetSectionsByIDs(ctx, allSectionIDs)
	if err != nil {
		return "", errors.WithStack(err)
	}

	maxTotalWords := c.maxTotalWords
	if maxTotalWords <= 0 {
		maxTotalWords = 50000
	}
	totalWords := 0

	var sb strings.Builder
	sb.WriteString("## Query\n\n")
	sb.WriteString(query)
	sb.WriteString("\n\n## Documents\n\n")

	for _, r := range results {
		for _, s := range r.Sections {
			section, exists := sectionsMap[s]
			if !exists {
				continue
			}

			if totalWords >= maxTotalWords {
				break
			}

			content, err := section.Content()
			if err != nil {
				return "", errors.WithStack(err)
			}

			sb.WriteString("### Document ")
			sb.WriteString(string(section.ID()))
			sb.WriteString("\n\n")

			words := strings.Fields(string(content))
			remaining := maxTotalWords - totalWords
			if len(words) > remaining {
				words = words[:remaining]
			}
			totalWords += len(words)

			sb.WriteString(strings.Join(words, " "))
			sb.WriteString("\n\n")
		}
	}

	return sb.String(), nil
}

var _ GroundingChecker = &LLMGroundingChecker{}
