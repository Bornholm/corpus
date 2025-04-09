package http

import (
	"net/http"
)

type Auth struct {
	Users []User
}

type User struct {
	Username string
	Password string
	Roles    []string
}
type Options struct {
	Address        string
	BaseURL        string
	Auth           Auth
	AllowAnonymous bool
	Mounts         map[string]http.Handler
}

type OptionFunc func(opts *Options)

func NewOptions(funcs ...OptionFunc) *Options {
	opts := &Options{
		Address: ":3002",
		BaseURL: "",
		Mounts:  map[string]http.Handler{},
	}
	for _, fn := range funcs {
		fn(opts)
	}
	return opts
}

func WithMount(prefix string, handler http.Handler) OptionFunc {
	return func(opts *Options) {
		opts.Mounts[prefix] = handler
	}
}

func WithBaseURL(baseURL string) OptionFunc {
	return func(opts *Options) {
		opts.BaseURL = baseURL
	}
}

func WithAddress(addr string) OptionFunc {
	return func(opts *Options) {
		opts.Address = addr
	}
}

func WithAllowAnonymous(allowed bool) OptionFunc {
	return func(opts *Options) {
		opts.AllowAnonymous = allowed
	}
}

func WithAuth(users ...User) OptionFunc {
	return func(opts *Options) {
		opts.Auth.Users = users
	}
}
