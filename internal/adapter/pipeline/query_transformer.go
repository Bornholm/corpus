package pipeline

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
)

// Hypothetical document
type HyDEQueryTransformer struct {
	llm   llm.Client
	store port.Store
}

const defaultHyDEPromptTemplate = `
As a knowledgeable and helpful research assistant, your task is to provide informative informations about the given context. Use your extensive knowledge base to offer clear, concise, and accurate responses to the user's inquiries.

Do not output anything than your answer.

## Topic

{{ .Query }}

## Context

This this the available collections of documents in the database. Use them to orient your answer.

{{ range .Collections }}
### {{ .Name }}

{{ .Description }}
{{ end }}
`

// TransformQuery implements QueryTransformer.
func (t *HyDEQueryTransformer) TransformQuery(ctx context.Context, query string) (string, error) {
	collections, err := t.store.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		return "", errors.WithStack(err)
	}

	prompt, err := llm.PromptTemplate(defaultHyDEPromptTemplate, struct {
		Query       string
		Collections []model.Collection
	}{
		Query:       query,
		Collections: collections,
	})
	if err != nil {
		return "", errors.WithStack(err)
	}

	completion, err := t.llm.ChatCompletion(ctx,
		llm.WithMessages(
			llm.NewMessage(llm.RoleUser, prompt),
		),
		llm.WithTemperature(0.2),
	)
	if err != nil {
		return "", errors.WithStack(err)
	}

	answer := completion.Message().Content()

	slog.DebugContext(ctx, "generated hypothetic answer", slog.String("answer", answer))

	return answer, nil
}

func NewHyDEQueryTransformer(client llm.Client, store port.Store) *HyDEQueryTransformer {
	return &HyDEQueryTransformer{
		llm:   client,
		store: store,
	}
}

var _ QueryTransformer = &HyDEQueryTransformer{}
