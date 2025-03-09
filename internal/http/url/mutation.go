package url

import (
	"errors"
	"net/url"
	"path/filepath"
	"slices"
)

type URL = url.URL

var Parse = url.Parse

func Mutate(u *url.URL, funcs ...MutationFunc) *url.URL {
	cloned := clone(u)

	for _, fn := range funcs {
		fn(cloned)
	}

	return cloned
}

type MutationFunc func(u *url.URL)

func keyValuesToValues(kv []string) url.Values {
	if len(kv)%2 != 0 {
		panic(errors.New("expected pair number of key/values"))
	}

	values := make(url.Values)

	var key string
	for idx := range kv {
		if idx%2 == 0 {
			key = kv[idx]
			continue
		}

		values.Add(key, kv[idx])
	}

	return values
}

func WithValues(kv ...string) MutationFunc {
	values := keyValuesToValues(kv)

	return func(u *url.URL) {
		query := u.Query()

		for k, vv := range values {
			for _, v := range vv {
				query.Add(k, v)
			}
		}

		u.RawQuery = query.Encode()
	}
}

func WithValuesReset() MutationFunc {
	return func(u *url.URL) {
		u.RawQuery = ""
	}
}

func WithoutValues(kv ...string) MutationFunc {
	toDelete := keyValuesToValues(kv)

	return func(u *url.URL) {
		query := u.Query()

		for keyToDelete, valuesToDelete := range toDelete {
			values, keyExists := query[keyToDelete]
			if !keyExists {
				continue
			}

			for _, d := range valuesToDelete {
				if d == "*" {
					query.Del(keyToDelete)
					break
				}

				query[keyToDelete] = slices.DeleteFunc(values, func(value string) bool {
					return value == d
				})
			}
		}
		u.RawQuery = query.Encode()
	}
}

func WithPath(paths ...string) MutationFunc {
	return func(u *url.URL) {
		u.Path = filepath.Join(paths...)
	}
}

func applyURLMutations(u *url.URL, funcs []MutationFunc) {
	for _, fn := range funcs {
		fn(u)
	}
}

func clone[T any](v *T) *T {
	copy := *v
	return &copy
}
