package httputil

import (
	"context"
	"crypto/sha256"
	"net/http"

	"github.com/diamondburned/gotkit/app"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
)

type ctxKey uint8

const (
	_ ctxKey = iota
	httpKey
	shouldCacheKey
)

// WithClient overrides the default HTTP client used by imgutil's HTTP
// functions. If ctx has an *Application instance and cache is true, then the
// Transport is wrapped.
func WithClient(ctx context.Context, cache bool, c *http.Client) context.Context {
	if cache {
		ctx = context.WithValue(ctx, shouldCacheKey, true)
	}

	return context.WithValue(ctx, httpKey, c)
}

// FromContext loads a client from the context and optionally injects the cache
// with the given namespace.
func FromContext(ctx context.Context, client *http.Client, cache string) *http.Client {
	if cli, ok := ctx.Value(httpKey).(*http.Client); ok {
		client = cli
	}

	if should, ok := ctx.Value(shouldCacheKey).(bool); !ok || should {
		client = injectCache(ctx, client, cache)
	}

	return client
}

// injectCache injects cache into the returned copy of a http.Client.
func injectCache(ctx context.Context, client *http.Client, cache string) *http.Client {
	app := app.FromContext(ctx)
	if app == nil {
		return client
	}

	cpy := *client
	cpy.Transport = &httpcache.Transport{
		Cache:     diskcache.New(app.CachePath(cache)),
		Transport: cpy.Transport,
	}

	return &cpy
}

// HashURL ensures that keys in the invalidURLs map are always 24 bytes long to
// reduce the length that each key takes. This puts additional (but minimal)
// strain on the GC.
func HashURL(url string) [sha256.Size224]byte {
	return sha256.Sum224([]byte(url))
}
