package httputil

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
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

	if cache != "" {
		if should, ok := ctx.Value(shouldCacheKey).(bool); !ok || should {
			client = injectCache(ctx, client, cache)
		}
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

// Some interesting benchmark results:
//
//    cpu: Intel(R) Core(TM) i5-8250U CPU @ 1.60GHz
//    BenchmarkMD5-8             523683              2185 ns/op         468.58 MB/s
//    BenchmarkSHA1-8            583852              1835 ns/op         558.12 MB/s
//    BenchmarkSHA224-8          301488              4047 ns/op         253.03 MB/s
//    BenchmarkSHA256-8          272781              4051 ns/op         252.78 MB/s
//
// SHA1 is actually faster than MD5 on this CPU, likely because of AVX2.

// HashURL hashes the given URL into a 28-byte string.
func HashURL(url string) string {
	hash := sha1.Sum([]byte(url))
	return base64.URLEncoding.EncodeToString(hash[:])
}
