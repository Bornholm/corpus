package client

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type RateLimitTransport struct {
	Base       http.RoundTripper
	MaxRetries int
}

func (t *RateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.Base
	if transport == nil {
		transport = http.DefaultTransport
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		resp, err = transport.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		waitTime := t.getWaitTime(resp)

		slog.DebugContext(req.Context(), "rate limited (429)", slog.Duration("wait_time", waitTime), slog.Int("attempt", attempt+1), slog.Int("max_retries", t.MaxRetries))

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if attempt == t.MaxRetries {
			break
		}

		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(waitTime):
		}

		if req.GetBody != nil {
			newBody, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to rewind request body: %w", err)
			}
			req.Body = newBody
		} else if req.Body != nil {
			return nil, fmt.Errorf("cannot retry request with one-time reader body")
		}
	}

	return resp, nil
}

func (t *RateLimitTransport) getWaitTime(resp *http.Response) time.Duration {
	defaultWait := time.Second

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter != "" {
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			return time.Duration(seconds) * time.Second
		}
		if date, err := http.ParseTime(retryAfter); err == nil {
			return time.Until(date)
		}
	}

	resetHeader := resp.Header.Get("X-RateLimit-Reset")
	if resetHeader != "" {
		if resetTime, err := strconv.ParseInt(resetHeader, 10, 64); err == nil {
			wait := time.Until(time.Unix(resetTime, 0))
			if wait > 0 {
				return wait
			}
		}
	}

	return defaultWait
}
