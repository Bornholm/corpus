package pipeline

import (
	"context"
	"slices"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
)

// Hypothetical document
type JudgeResultsTransformer struct {
	llm   llm.Client
	store port.Store
}

const defaultJudgeResultsTransformer = `
As a knowledgeable and helpful research assistant, your task is to judge which one of the documents given to you in context are relevant to the following query.

Respond as JSON.

## Query

{{ .Query }}
`

// TransformResults implements ResultsTransformer.
func (t *JudgeResultsTransformer) TransformResults(ctx context.Context, query string, results []*port.IndexSearchResult) ([]*port.IndexSearchResult, error) {
	systemPrompt, err := llm.PromptTemplate(defaultJudgeResultsTransformer, struct {
		Query string
	}{
		Query: query,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	userPrompt, err := t.getUserPrompt(ctx, results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	completion, err := t.llm.ChatCompletion(ctx,
		llm.WithJSONResponse(
			llm.NewResponseSchema(
				"FilteredResults",
				"The list of documents identifiers that are relevant to the query",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"documents": map[string]any{
							"type":        "array",
							"description": "The list of document's identifiers relevant to the query",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
					"required":             []string{"documents"},
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
		Documents []string `json:"documents"`
	}

	documents, err := llm.ParseJSON[llmResponse](completion.Message())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	selected := map[model.SectionID]struct{}{}
	for _, d := range documents {
		for _, s := range d.Documents {
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

func (t *JudgeResultsTransformer) getUserPrompt(ctx context.Context, results []*port.IndexSearchResult) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Documents\n\n")

	for _, r := range results {
		for _, s := range r.Sections {
			section, err := t.store.GetSectionBySourceAndID(ctx, r.Source, s)
			if err != nil {
				if errors.Is(err, port.ErrNotFound) {
					continue
				}

				return "", errors.WithStack(err)
			}

			sb.WriteString("## ")
			sb.WriteString(string(section.ID()))
			sb.WriteString("\n\n")
			sb.WriteString(string(section.Content()))
		}
	}

	return sb.String(), nil
}

func NewJudgeResultsTransformer(client llm.Client, store port.Store) *JudgeResultsTransformer {
	return &JudgeResultsTransformer{
		llm:   client,
		store: store,
	}
}

var _ ResultsTransformer = &JudgeResultsTransformer{}
