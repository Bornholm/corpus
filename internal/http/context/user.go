package context

import (
	"context"

	"github.com/bornholm/corpus/pkg/model"
)

const keyUser contextKey = "user"

func User(ctx context.Context) model.User {
	user, ok := ctx.Value(keyUser).(model.User)
	if !ok {
		return nil
	}

	return user
}

func SetUser(ctx context.Context, user model.User) context.Context {
	return context.WithValue(ctx, keyUser, user)
}
