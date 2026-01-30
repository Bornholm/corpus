package llm

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

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

// Wait blocks until the request can be processed.
//
// High Priority: Blocks until 'n' tokens are available (standard behavior).
// Low Priority:  Blocks until the bucket has enough tokens to cover 'n' PLUS the threshold buffer.
func (pl *PriorityLimiter) Wait(ctx context.Context, n int, isHighPriority bool) error {
	// 1. High Priority Path: Standard behavior
	if isHighPriority {
		return pl.limiter.WaitN(ctx, n)
	}

	// 2. Low Priority Path: Check-Reserve-Cancel Loop
	// We need to ensure we don't consume tokens unless the bucket is healthy.

	burst := float64(pl.limiter.Burst())
	// The number of tokens we need to see in the bucket to allow a low-prio request
	requiredTokens := (burst * pl.lowPrioThreshold) + float64(n)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// A. Peek at current tokens (Optimization to avoid expensive Reservation/Cancel cycles)
		// If we are obviously empty, just wait a bit without locking anything.
		currentTokens := pl.limiter.Tokens()
		if currentTokens < requiredTokens {
			// Calculate rough wait time to reach target
			missing := requiredTokens - currentTokens
			waitDuration := time.Duration(float64(time.Second) * (missing / float64(pl.limiter.Limit())))

			// Force a small minimum wait to prevent hot-looping (CPU spinning)
			if waitDuration < 10*time.Millisecond {
				waitDuration = 10 * time.Millisecond
			}

			// Wait either for the calculated time or context cancel
			select {
			case <-time.After(waitDuration):
				continue // Check again
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// B. Strict Check using Reserve
		// We believe we have space, but we need to prove it atomically.
		// We reserve 'n' tokens.
		r := pl.limiter.ReserveN(time.Now(), n)
		if !r.OK() {
			// n exceeds burst size; this request is impossible
			return fmt.Errorf("request exceeds limiter burst")
		}

		// C. Analyze the Reservation
		// r.Delay() tells us how long we must wait to get 'n'.
		// If r.Delay() > 0, it means the bucket didn't actually have 'n' tokens ready *right now*.
		// Since we are Low Priority, we generally don't want to wait in line;
		// we want to ensure the bucket was ALREADY full enough.

		// Note: The logic below implies Low Priority only passes if NO waiting is required
		// relative to the tokens available.

		if r.Delay() > 0 {
			// The bucket was not full enough to give us 'n' immediately.
			// This implies we are eating into "fresh" tokens, which might starve VIPs.
			// REJECT logic: Put the tokens back and wait.
			r.Cancel()

			select {
			case <-time.After(100 * time.Millisecond): // Backoff
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Success! The tokens were available immediately.
		// However, we still need to respect the Rate Limiter's timing if there was a tiny delay,
		// though with Delay() == 0 there is no sleep.
		time.Sleep(r.Delay())
		return nil
	}
}
