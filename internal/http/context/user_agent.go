package context

import (
	"context"
)

const keyIsDesktopApp contextKey = "isDesktopApp"

func IsDesktopApp(ctx context.Context) bool {
	isDesktopApp, ok := ctx.Value(keyIsDesktopApp).(bool)
	if !ok {
		return false
	}

	return isDesktopApp
}

func SetDesktopApp(ctx context.Context, isDesktopApp bool) context.Context {
	return context.WithValue(ctx, keyIsDesktopApp, isDesktopApp)
}
