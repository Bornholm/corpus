package pipeline

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/log"
	"github.com/bornholm/corpus/internal/text"
	"github.com/bornholm/genai/llm"
	"github.com/bornholm/genai/llm/prompt"
	"github.com/pkg/errors"
)

// Hypothetical document
type JudgeResultsTransformer struct {
	llm      llm.Client
	store    port.DocumentStore
	maxWords int
}

const defaultJudgeResultsTransformer = `
You are a document retrieval system that evaluates document relevance against a user query. Your task is to analyze the provided documents and identify only those that are relevant to the query.

For each document, consider:
1. Topical alignment with the query's main subject
2. Information that directly answers or addresses the query
3. Content that provides useful context or supporting details for the query

Return your assessment as a structured JSON object containing ONLY the identifiers of relevant documents. Do not include explanations, document content, or any other information in your response.

For example:
{"identifiers": ["doc_123", "doc_456"], "explanation": "The doc_123 contains informations about ..., relevant to the query"}

If no documents are relevant, return:
{"identifiers": [], "explanation": "No document provide information linked to the query"}
`

// TransformResults implements ResultsTransformer.
func (t *JudgeResultsTransformer) TransformResults(ctx context.Context, query string, results []*port.IndexSearchResult, opts port.IndexSearchOptions) ([]*port.IndexSearchResult, error) {
	systemPrompt, err := prompt.Template(defaultJudgeResultsTransformer, struct {
	}{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	userPrompt, err := t.getUserPrompt(ctx, query, results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	seed, err := text.IntHash(systemPrompt + query)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ctx = log.WithAttrs(ctx, slog.Int("seed", seed))

	completion, err := t.llm.ChatCompletion(ctx,
		llm.WithJSONResponse(
			llm.NewResponseSchema(
				"FilteredResults",
				"The list of document's identifiers that are relevant to the query",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"identifiers": map[string]any{
							"type":        "array",
							"description": "The list of document's identifiers relevant to the query",
							"items": map[string]any{
								"type": "string",
							},
						},
						"explanation": map[string]any{
							"type":        "string",
							"description": "An explanation of why you selected theses documents or not.",
						},
					},
					"required":             []string{"identifiers", "explanation"},
					"additionalProperties": false,
				},
			),
		),
		llm.WithMessages(
			llm.NewMessage(llm.RoleSystem, systemPrompt),
			llm.NewMessage(llm.RoleUser, userPrompt),
		),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	type llmResponse struct {
		Identifiers []string `json:"identifiers"`
		Explanation string   `json:"explanation"`
	}

	responses, err := llm.ParseJSON[llmResponse](completion.Message())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	slog.DebugContext(ctx, "judge responses", slog.Any("responses", responses))

	selected := map[model.SectionID]struct{}{}
	for _, r := range responses {
		for _, s := range r.Identifiers {
			selected[model.SectionID(s)] = struct{}{}
		}
	}

	for _, r := range results {
		r.Sections = slices.Collect(func(yield func(model.SectionID) bool) {
			for _, sectionID := range r.Sections {
				if _, exists := selected[sectionID]; !exists {
					continue
				}

				if !yield(sectionID) {
					return
				}
			}
		})
	}

	results = slices.Collect(func(yield func(r *port.IndexSearchResult) bool) {
		for _, r := range results {
			if len(r.Sections) == 0 {
				continue
			}

			if !yield(r) {
				return
			}
		}
	})

	return results, nil
}

func (t *JudgeResultsTransformer) getUserPrompt(ctx context.Context, query string, results []*port.IndexSearchResult) (string, error) {
	var sb strings.Builder
	sb.WriteString("## Query\n\n")
	sb.WriteString(query)
	sb.WriteString("\n\n")

	sb.WriteString("## Documents\n\n")

	for _, r := range results {
		for _, s := range r.Sections {
			section, err := t.store.GetSectionByID(ctx, s)
			if err != nil {
				return "", errors.WithStack(err)
			}

			sb.WriteString("### Document ")
			sb.WriteString(string(section.ID()))
			sb.WriteString("\n\n")

			sb.WriteString("**Identifier:**")
			sb.WriteString(string(section.ID()))
			sb.WriteString("\n\n")

			content, err := section.Content()
			if err != nil {
				return "", errors.WithStack(err)
			}

			sb.WriteString(string(content))

			sb.WriteString("\n\n")
		}
	}

	return sb.String(), nil
}

func NewJudgeResultsTransformer(client llm.Client, store port.DocumentStore, maxWords int) *JudgeResultsTransformer {
	return &JudgeResultsTransformer{
		llm:      client,
		store:    store,
		maxWords: maxWords,
	}
}

var _ ResultsTransformer = &JudgeResultsTransformer{}
