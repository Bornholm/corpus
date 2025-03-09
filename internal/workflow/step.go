package workflow

import "context"

type Step interface {
	Execute(ctx context.Context) error
	Compensate(ctx context.Context) error
}

type step struct {
	execute    func(ctx context.Context) error
	compensate func(ctx context.Context) error
}

// Compensate implements Step.
func (s *step) Compensate(ctx context.Context) error {
	if s.compensate == nil {
		return nil
	}

	return s.compensate(ctx)
}

// Execute implements Step.
func (s *step) Execute(ctx context.Context) error {
	if s.execute == nil {
		return nil
	}

	return s.execute(ctx)
}

var _ Step = &step{}

func StepFunc(execute func(ctx context.Context) error, compensate func(ctx context.Context) error) Step {
	return &step{
		execute:    execute,
		compensate: compensate,
	}
}
