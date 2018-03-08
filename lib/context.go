package lib

import (
	"context"

	"github.com/jinzhu/gorm"
	"go.uber.org/zap"
)

type ctxKey string

const (
	ctxKeyLogger ctxKey = "logger"
	ctxKeyDB     ctxKey = "db"
)

// GetLogger returns the logger associated with the context. If no logger has been attached
// (see WithContext), it returns the global logger - zap.L().
func GetLogger(ctx context.Context) *zap.Logger {
	l, ok := ctx.Value(ctxKeyLogger).(*zap.Logger)
	if !ok {
		return zap.L()
	}
	return l
}

// WithLogger returns a context with the given logger attached.
func WithLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, l)
}

// WithNamedLogger is a shorthand to calling WithLogger(ctx, GetLogger(ctx).Named(name)).
func WithNamedLogger(ctx context.Context, name string) context.Context {
	return WithLogger(ctx, GetLogger(ctx).Named(name))
}

// GetDB returns the GORM DB associated with the context, or nil.
func GetDB(ctx context.Context) *gorm.DB {
	db, _ := ctx.Value(ctxKeyDB).(*gorm.DB)
	return db
}

// WithDB returns a context with the given DB attached.
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, ctxKeyDB, db)
}
