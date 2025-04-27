package backend

import "errors"

var (
	ErrSchemeNotRegistered = errors.New("scheme was not registered")
	ErrNotImplemented      = errors.New("not implemented")
)
