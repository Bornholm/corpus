package service

import (
	"context"
	"log/slog"

	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/pkg/errors"
)

// WithQueryReformulator enables the iterative re-retrieval loop (Phase 2): when
// the grounding verifier is not confident, the query is reformulated and search
// is retried before answering. Requires a GroundingChecker to gate the loop.
func WithQueryReformulator(reformulator QueryReformulator) DocumentManagerOptionFunc {
	return func(opts *DocumentManagerOptions) {
		opts.QueryReformulator = reformulator
	}
}

// WithQueryDecomposer enables query decomposition (Phase 3): a complex question
// is split into sub-questions, each searched independently and their evidence
// fused before answering.
func WithQueryDecomposer(decomposer QueryDecomposer) DocumentManagerOptionFunc {
	return func(opts *DocumentManagerOptions) {
		opts.QueryDecomposer = decomposer
	}
}

// WithIterativeMaxRounds caps the number of re-retrieval rounds (default 1).
func WithIterativeMaxRounds(rounds int) DocumentManagerOptionFunc {
	return func(opts *DocumentManagerOptions) {
		opts.IterativeMaxRounds = rounds
	}
}

// AskResult is the outcome of AskWithRetrieval: the generated (or abstention)
// answer, the section contents used, the final fused result set (for source
// display), the grounding verdict (nil when the checker is disabled) and the
// number of extra re-retrieval rounds performed.
type AskResult struct {
	Answer    string
	Contents  map[model.SectionID]string
	Results   []*port.IndexSearchResult
	Grounding *GroundingResult
	Rounds    int
}

// abstentionAnswer builds the user-facing message returned when the grounding
// verifier judges the evidence insufficient.
func abstentionAnswer(grounding *GroundingResult) string {
	message := defaultAbstentionMessage
	if grounding != nil && grounding.Explanation != "" {
		message += " (" + grounding.Explanation + ")"
	}
	return message
}

// AskWithRetrieval is the orchestration layer that unifies retrieval and
// answering, adding the MothRAG-derived mechanisms on top of the single-shot
// Search+Ask path:
//
//   - query decomposition (Phase 3): when a QueryDecomposer is configured, the
//     original query and its sub-questions are searched and their evidence fused;
//   - iterative re-retrieval (Phase 2): when a QueryReformulator is configured
//     and the grounding verdict is not confident, the query is reformulated and
//     searched again (up to iterativeMaxRounds), enlarging the evidence set;
//   - grounding gate (Phase 1): the final verdict drives abstention.
//
// With none of these configured it degrades to a plain Search followed by
// answer generation, matching the legacy Search+Ask behaviour.
func (m *DocumentManager) AskWithRetrieval(ctx context.Context, query string, collections []model.CollectionID, funcs ...DocumentManagerAskOptionFunc) (*AskResult, error) {
	askOpts := NewDocumentManagerAskOptions(funcs...)

	systemPromptTemplate := askOpts.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = defaultSystemPromptTemplate
	}

	searchFuncs := make([]DocumentManagerSearchOptionFunc, 0, 1)
	if len(collections) > 0 {
		searchFuncs = append(searchFuncs, WithDocumentManagerSearchCollections(collections...))
	}

	// Round 0: (optionally decomposed) retrieval.
	results, err := m.retrieve(ctx, query, searchFuncs)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	result := &AskResult{}

	// Grounding-gated iterative re-retrieval.
	var grounding *GroundingResult
	rounds := 0
	if m.groundingChecker != nil {
		for {
			if len(results) == 0 {
				break
			}

			grounding, err = m.groundingChecker.Check(ctx, query, results)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			confident := grounding.Status == GroundingValid && grounding.Score >= m.groundingMinScore
			if confident || m.queryReformulator == nil || rounds >= m.iterativeMaxRounds {
				break
			}

			reformulated, err := m.queryReformulator.Reformulate(ctx, query, grounding.Explanation)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			if reformulated == "" || reformulated == query {
				break
			}

			rounds++
			slog.InfoContext(ctx, "iterative re-retrieval",
				slog.String("reformulated_query", reformulated),
				slog.Int("round", rounds),
			)

			more, err := m.retrieve(ctx, reformulated, searchFuncs)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			results = fuseResults(results, more)
		}
	}

	result.Grounding = grounding
	result.Rounds = rounds
	result.Results = results

	// No evidence at all: leave it to the caller (no-results handling).
	if len(results) == 0 {
		return result, nil
	}

	// Grounding gate → abstain instead of generating an unsupported answer.
	if grounding != nil && (grounding.Status == GroundingInvalid || grounding.Score < m.groundingMinScore) {
		slog.InfoContext(ctx, "abstaining: retrieved evidence is not sufficiently grounded",
			slog.String("status", string(grounding.Status)),
			slog.Float64("score", grounding.Score),
			slog.Float64("min_score", m.groundingMinScore),
			slog.Int("rounds", rounds),
		)

		result.Answer = abstentionAnswer(grounding)
		result.Contents = map[model.SectionID]string{}
		return result, nil
	}

	answer, contents, err := m.generateResponse(ctx, systemPromptTemplate, query, results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	result.Answer = answer
	result.Contents = contents

	return result, nil
}

// retrieve performs a single retrieval step. When a QueryDecomposer is
// configured, it searches the original query plus each sub-question and fuses
// the evidence; otherwise it is a plain Search.
func (m *DocumentManager) retrieve(ctx context.Context, query string, searchFuncs []DocumentManagerSearchOptionFunc) ([]*port.IndexSearchResult, error) {
	if m.queryDecomposer == nil {
		results, err := m.Search(ctx, query, searchFuncs...)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return results, nil
	}

	subQueries, err := m.queryDecomposer.Decompose(ctx, query)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Always include the original query, then each distinct sub-question.
	queries := make([]string, 0, len(subQueries)+1)
	queries = append(queries, query)
	for _, sq := range subQueries {
		if sq == query {
			continue
		}
		queries = append(queries, sq)
	}

	fused := make([]*port.IndexSearchResult, 0)
	for _, q := range queries {
		r, err := m.Search(ctx, q, searchFuncs...)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		fused = fuseResults(fused, r)
	}

	return fused, nil
}

// fuseResults unions several result groups, de-duplicating at the section level
// (a section already contributed by an earlier group is dropped) and discarding
// results left with no sections. Input slices are not mutated.
func fuseResults(groups ...[]*port.IndexSearchResult) []*port.IndexSearchResult {
	seen := map[model.SectionID]struct{}{}
	out := make([]*port.IndexSearchResult, 0)

	for _, group := range groups {
		for _, r := range group {
			kept := make([]model.SectionID, 0, len(r.Sections))
			for _, sectionID := range r.Sections {
				if _, exists := seen[sectionID]; exists {
					continue
				}
				seen[sectionID] = struct{}{}
				kept = append(kept, sectionID)
			}

			if len(kept) == 0 {
				continue
			}

			out = append(out, &port.IndexSearchResult{
				Source:   r.Source,
				Sections: kept,
			})
		}
	}

	return out
}
