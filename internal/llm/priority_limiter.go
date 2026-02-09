package llm

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type PriorityLimiter struct {
	limiter          *rate.Limiter
	lowPrioThreshold float64

	lowPrioMutex sync.Mutex
}

func NewPriorityLimiter(r rate.Limit, b int, lowPrioThreshold float64) *PriorityLimiter {
	return &PriorityLimiter{
		limiter:          rate.NewLimiter(r, b),
		lowPrioThreshold: lowPrioThreshold,
	}
}

func (pl *PriorityLimiter) Wait(ctx context.Context, n int, isHighPriority bool) error {
	if isHighPriority {
		return pl.limiter.WaitN(ctx, n)
	}

	threshold := float64(pl.limiter.Burst()) * pl.lowPrioThreshold
	requiredTokens := float64(n) + threshold

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		currentTokens := pl.limiter.Tokens()

		slog.DebugContext(ctx, "waiting for slot", slog.Int("burst", pl.limiter.Burst()), slog.Float64("current_tokens", currentTokens), slog.Float64("required_tokens", requiredTokens))

		if currentTokens < requiredTokens {
			missing := requiredTokens - currentTokens
			waitDuration := time.Duration(missing / float64(pl.limiter.Limit()) * float64(time.Second))

			if waitDuration < 100*time.Millisecond {
				waitDuration = 100 * time.Millisecond
			}

			select {
			case <-time.After(waitDuration):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		r := pl.limiter.ReserveN(time.Now(), n)

		if !r.OK() {
			return fmt.Errorf("request exceeds limiter burst")
		}

		if r.Delay() > 0 {
			r.Cancel()
			select {
			case <-time.After(250 * time.Millisecond):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	}
}
