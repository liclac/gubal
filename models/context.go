package models

import (
	"context"
)

type ctxKey string

const ctxKeyDataStore ctxKey = "data_store"

// GetDataStore returns the data store associated with the context, or nil.
func GetDataStore(ctx context.Context) DataStore {
	ds, _ := ctx.Value(ctxKeyDataStore).(DataStore)
	return ds
}

// WithDataStore returns a context with the given data store attached.
func WithDataStore(ctx context.Context, ds DataStore) context.Context {
	return context.WithValue(ctx, ctxKeyDataStore, ds)
}
