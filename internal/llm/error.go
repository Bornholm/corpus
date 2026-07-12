package llm

import (
	"net/http"

	"github.com/bornholm/genai/llm"
	"github.com/pkg/errors"
)

// IsRateLimit reports whether err is a rate-limiting (HTTP 429) signal from an
// LLM provider.
//
// It exists because github.com/bornholm/genai's *llm.HTTPError does not
// implement an Is method: despite the package documentation, a 429 HTTPError
// does NOT satisfy errors.Is(err, llm.ErrRateLimit). We therefore also inspect
// the wrapped HTTPError status code explicitly.
//
// Unlike llm.IsRetryable, this predicate is narrow: it matches only rate limits,
// not 5xx server errors, so callers can safely map it to a "service overloaded"
// (503) response without masking genuine upstream failures.
func IsRateLimit(err error) bool {
	if errors.Is(err, llm.ErrRateLimit) {
		return true
	}

	var httpErr *llm.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusTooManyRequests
}
