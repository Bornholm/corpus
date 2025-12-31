package common

import "net/http"

type Error struct {
	err         string
	userMessage string
	statusCode  int
}

// StatusCode implements HTTPError.
func (e *Error) StatusCode() int {
	return e.statusCode
}

// Error implements UserFacingError.
func (e *Error) Error() string {
	return e.err
}

// UserMessage implements UserFacingError.
func (e *Error) UserMessage() string {
	return e.userMessage
}

func NewError(err string, userMessage string, statusCode int) *Error {
	return &Error{err, userMessage, statusCode}
}

var _ UserFacingError = &Error{}
var _ HTTPError = &Error{}

func NewHTTPError(statusCode int) *Error {
	return &Error{http.StatusText(statusCode), http.StatusText(statusCode), statusCode}
}
