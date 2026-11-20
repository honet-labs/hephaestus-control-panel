package domain

import "context"

type actorContextKey struct{}
type ClientIPContextKey struct{}

func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, actorContextKey{}, user)
}

func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(actorContextKey{}).(*User)
	return user
}
