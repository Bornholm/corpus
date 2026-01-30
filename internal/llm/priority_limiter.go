package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type PriorityLimiter struct {
	limiter          *rate.Limiter
	lowPrioThreshold float64

	lowPrioMutex sync.Mutex
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

func (pl *PriorityLimiter) Wait(ctx context.Context, n int, isHighPriority bool) error {
	if isHighPriority {
		return pl.limiter.WaitN(ctx, n)
	}

	// We lock here to prevent "Thundering Herd" race conditions where
	// 5 indexers all see the bucket is full and drain it instantly.
	pl.lowPrioMutex.Lock()
	defer pl.lowPrioMutex.Unlock()

	burst := float64(pl.limiter.Burst())
	requiredTokens := (burst * pl.lowPrioThreshold) + float64(n)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		currentTokens := pl.limiter.Tokens()
		if currentTokens < requiredTokens {
			missing := requiredTokens - currentTokens
			waitDuration := time.Duration(float64(time.Second) * (missing / float64(pl.limiter.Limit())))

			if waitDuration < 10*time.Millisecond {
				waitDuration = 10 * time.Millisecond
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
			case <-time.After(100 * time.Millisecond): // Backoff
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		time.Sleep(r.Delay())
		return nil
	}
}
