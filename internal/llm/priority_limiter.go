package llm

import "golang.org/x/time/rate"

type PriorityLimiter struct {
	limiter          *rate.Limiter
	lowPrioThreshold float64
}

func NewPriorityLimiter(r rate.Limit, b int, threshold float64) *PriorityLimiter {
	return &PriorityLimiter{
		limiter:          rate.NewLimiter(r, b),
		lowPrioThreshold: threshold,
	}
}

func (pl *PriorityLimiter) Allow(isHighPriority bool) bool {
	currentTokens := pl.limiter.Tokens()
	maxBurst := float64(pl.limiter.Burst())

	if !isHighPriority {
		if currentTokens < (maxBurst * pl.lowPrioThreshold) {
			return false
		}
	}

	return pl.limiter.Allow()
}
