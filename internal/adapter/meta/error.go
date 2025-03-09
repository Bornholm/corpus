package meta

import (
	"strings"
)

type AggregatedError struct {
	errs []error
}

func (e *AggregatedError) Error() string {
	var sb strings.Builder

	sb.WriteString("aggregated error: ")

	for i, e := range e.errs {
		if i > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(e.Error())
	}

	return sb.String()
}

func (e *AggregatedError) Add(errs ...error) {
	e.errs = append(e.errs, errs...)
}

func (e *AggregatedError) Len() int {
	return len(e.errs)
}

func NewAggregatedError(errs ...error) *AggregatedError {
	return &AggregatedError{errs}
}
