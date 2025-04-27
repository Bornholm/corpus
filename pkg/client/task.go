package client

import (
	"fmt"
	"time"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/http/handler/api"
	"github.com/pkg/errors"
)

type WaitForOptions struct {
	PollInterval time.Duration
}

type WaitForOptionFunc func(opts *WaitForOptions)

func WithWaitForPollInterval(interval time.Duration) WaitForOptionFunc {
	return func(opts *WaitForOptions) {
		opts.PollInterval = interval
	}
}

func NewWaitForOptions(funcs ...WaitForOptionFunc) *WaitForOptions {
	opts := &WaitForOptions{
		PollInterval: time.Second * 2,
	}

	for _, fn := range funcs {
		fn(opts)
	}

	return opts
}

func (c *Client) WaitFor(taskID port.TaskID, funcs ...WaitForOptionFunc) (*api.Task, error) {
	opts := NewWaitForOptions(funcs...)

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	endpoint := fmt.Sprintf("/tasks/%s", taskID)

	for {
		var res api.ShowTaskResponse
		if err := c.jsonRequest("GET", endpoint, nil, nil, &res); err != nil {
			return nil, errors.WithStack(err)
		}

		if !res.Task.FinishedAt.IsZero() {
			return res.Task, nil
		}

		<-ticker.C
	}
}
