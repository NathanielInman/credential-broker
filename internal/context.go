package internal

import (
	"context"

	"github.com/nate/credential-broker/store"
)

type contextKey string

const (
	CtxSecret contextKey = "secret"
	CtxKey    contextKey = "key"
	CtxUser   contextKey = "user"
	CtxName   contextKey = "name"
	CtxEmail  contextKey = "email"
	CtxIP     contextKey = "ip"
	ctxBody   contextKey = "body"
	ctxDB     contextKey = "db"
	ctxBroker contextKey = "broker"
)

func WithBody(ctx context.Context, body string) context.Context {
	return context.WithValue(ctx, ctxBody, body)
}

func GetBody(ctx context.Context) string {
	if v, ok := ctx.Value(ctxBody).(string); ok {
		return v
	}
	return ""
}

func WithServer(ctx context.Context, db *store.Store, broker interface{}) context.Context {
	ctx = context.WithValue(ctx, ctxDB, db)
	ctx = context.WithValue(ctx, ctxBroker, broker)
	return ctx
}
