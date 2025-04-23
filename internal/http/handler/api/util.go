package api

import (
	"net/url"
	"strconv"
)

func getQueryPage(query url.Values, defaultValue int) int {
	return getQueryInt(query, "page", defaultValue)
}

func getQueryLimit(query url.Values, defaultValue int) int {
	return getQueryInt(query, "limit", defaultValue)
}

func getQueryInt(query url.Values, name string, defaultValue int) int {
	raw := query.Get(name)
	if raw == "" {
		return defaultValue
	}

	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return defaultValue
	}

	return int(value)
}
