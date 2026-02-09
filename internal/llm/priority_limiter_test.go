package llm

import (
	"context"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestPriorityLimiter_Threshold_Enforcement(t *testing.T) {
	pl := NewPriorityLimiter(rate.Limit(1), 10, 0.5)

	pl.limiter.ReserveN(time.Now(), 7)

	ctxLow, cancelLow := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelLow()

	err := pl.Wait(ctxLow, 1, false)
	if err == nil {
		t.Error("Low priority request should have timed out due to low token bucket, but it succeeded")
	} else if ctxLow.Err() == nil {
		t.Errorf("Unexpected error for low priority: %v", err)
	}

	ctxHigh, cancelHigh := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelHigh()

	start := time.Now()
	err = pl.Wait(ctxHigh, 1, true)
	if err != nil {
		t.Fatalf("High priority request failed: %v", err)
	}

	if time.Since(start) > 20*time.Millisecond {
		t.Log("[WARN] High priority request took longer than expected (did it wait?)")
	}
}

func TestPriorityLimiter_Starvation(t *testing.T) {
	regenRate := 100 * time.Millisecond
	limiter := rate.NewLimiter(rate.Every(regenRate), 1)

	limiter.ReserveN(time.Now(), 1)

	pl := &PriorityLimiter{
		limiter:          limiter,
		lowPrioThreshold: 0.0,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	winner := make(chan string, 2)

	go func() {
		defer wg.Done()
		err := pl.Wait(context.Background(), 1, false)
		if err == nil {
			winner <- "background"
		}
	}()

	time.Sleep(10 * time.Millisecond)

	go func() {
		defer wg.Done()
		err := pl.Wait(context.Background(), 1, true)
		if err == nil {
			winner <- "user"
		}
	}()

	wg.Wait()
	close(winner)

	first := <-winner
	second := <-winner

	if first != "user" {
		t.Errorf("Priority failure: Expected 'user' to win the race, but '%s' won", first)
	}

	t.Logf("Race winner: %s, Follow up: %s", first, second)
}

func TestPriorityLimiter_GlobalRateLimit(t *testing.T) {
	limitRate := 100
	burst := 10

	limiter := rate.NewLimiter(rate.Limit(limitRate), burst)

	pl := &PriorityLimiter{
		limiter:          limiter,
		lowPrioThreshold: 0.5,
	}

	totalRequests := 200

	var wg sync.WaitGroup
	wg.Add(totalRequests)

	startTime := time.Now()

	for i := 0; i < totalRequests; i++ {
		isHighPriority := i%2 == 0

		go func(highPrio bool) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := pl.Wait(ctx, 1, highPrio)
			if err != nil {
				t.Errorf("Request failed: %v", err)
			}
		}(isHighPriority)
	}

	wg.Wait()
	duration := time.Since(startTime)

	expectedMinDuration := time.Duration(float64(totalRequests-burst)/float64(limitRate)) * time.Second

	conservativeMinDuration := time.Duration(float64(expectedMinDuration) * 0.90)

	t.Logf("Processed %d requests in %v", totalRequests, duration)
	t.Logf("Theoretical minimum time: %v", expectedMinDuration)

	if duration < conservativeMinDuration {
		t.Fatalf("Rate limit violated! Global throughput too high.\n"+
			"Expected at least: %v\n"+
			"Actual duration:   %v\n"+
			"This implies requests are bypassing the limiter.",
			conservativeMinDuration, duration)
	}
}
