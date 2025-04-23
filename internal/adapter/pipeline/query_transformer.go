package pipeline

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"log/slog"
	"strings"

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
As a knowledgeable and helpful research assistant, generate a hypothetical best-guess answer to the following query. Do not output anything than your answer.

## Query

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

	// seed, err := computeSeed(query)
	// if err != nil {
	// 	return "", errors.WithStack(err)
	// }

	// slog.DebugContext(ctx, "using seed", slog.Int("seed", seed))

	completion, err := t.llm.ChatCompletion(ctx,
		llm.WithMessages(
			llm.NewMessage(llm.RoleUser, prompt),
		),
		llm.WithTemperature(0.2),
		//llm.WithSeed(seed),
	)
	if err != nil {
		return "", errors.WithStack(err)
	}

	answer := completion.Message().Content()

	slog.DebugContext(ctx, "generated hypothetic answer", slog.String("answer", answer))

	var sb strings.Builder

	sb.WriteString(query)
	sb.WriteString("\n\n")
	sb.WriteString(answer)

	return sb.String(), nil
}

func NewHyDEQueryTransformer(client llm.Client, store port.Store) *HyDEQueryTransformer {
	return &HyDEQueryTransformer{
		llm:   client,
		store: store,
	}
}

var _ QueryTransformer = &HyDEQueryTransformer{}

func computeSeed(query string) (int, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	sum := md5.Sum([]byte(query))
	seed := binary.BigEndian.Uint32(sum[:])
	return int(seed), nil
}
