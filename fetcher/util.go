package fetcher

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"strings"

	"github.com/spf13/afero"
	"go.uber.org/zap"

	"github.com/liclac/gubal/lib"
)

type ctxKey string

const ctxKeyCacheFS ctxKey = "cache_fs"

// WithCacheFS associates a cache filesystem with the given context.
func WithCacheFS(ctx context.Context, fs afero.Fs) context.Context {
	return context.WithValue(ctx, ctxKeyCacheFS, fs)
}

// GetCacheFS returns the context's associated cache FS.
func GetCacheFS(ctx context.Context) afero.Fs {
	fs, _ := ctx.Value(ctxKeyCacheFS).(afero.Fs)
	return fs
}

func trim(s string) string {
	return strings.TrimSpace(s)
}

func doRequestWithCache(fs afero.Fs, key string, req *http.Request) (*http.Response, error) {
	if fs == nil {
		return http.DefaultClient.Do(req)
	}

	ctx := req.Context()
	filename := key + ".http"
	if f, err := fs.Open(filename); err == nil {
		lib.GetLogger(ctx).Info("Reusing a cached response", zap.String("filename", filename))
		return http.ReadResponse(bufio.NewReader(f), req)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		lib.GetLogger(ctx).Info("Writing response to cache", zap.String("filename", filename))

		var buf bytes.Buffer
		if err := resp.Write(&buf); err != nil {
			return nil, err
		}
		if err := afero.WriteFile(fs, filename, buf.Bytes(), 0644); err != nil {
			return nil, err
		}

		// Because resp.Write consumes the body and I'm a lazy arsehole.
		resp, err = http.ReadResponse(bufio.NewReader(&buf), req)
	default:
		lib.GetLogger(ctx).Info("Not caching unsuccessful response",
			zap.Stringer("url", req.URL),
			zap.Int("status", resp.StatusCode),
		)
	}
	return resp, nil
}
