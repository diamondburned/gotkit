package imgutil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/diamondburned/gotkit/gtkutil/httputil"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

// parallelMult * 4 = maxConcurrency
const parallelMult = 4

// parallel is used to throttle concurrent downloads.
var parallel = semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1)) * parallelMult)

var (
	fetchingURLs = map[string]*sync.Mutex{}
	fetchingMu   sync.Mutex

	// TODO: limit the size of invalidURLs.
	invalidURLs sync.Map
)

var errURLNotFound = errors.New("URL not found (cached)")

func urlIsInvalid(url string) bool {
	h := httputil.HashURL(url)

	vt, ok := invalidURLs.Load(h)
	if !ok {
		return false
	}

	t := time.Unix(vt.(int64), 0)
	if t.Add(time.Hour).After(time.Now()) {
		// fetched within the hour
		return true
	}

	invalidURLs.Delete(h)
	return false
}

func markURLInvalid(url string) {
	invalidURLs.Store(httputil.HashURL(url), time.Now().Unix())
}

func fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	if urlIsInvalid(url) {
		return nil, errURLNotFound
	}

	// How this works: we acquire a mutex for each request so that only 1
	// request per URL is ever sent. We will then perform the request so that
	// the cache is populated, and then repeat. This way, only 1 parallel
	// request per URL is ever done, but the ratio of cache hits is much higher.
	//
	// This isn't too bad, actually. Only the initial HTTP connection is done on
	// its own; the images will still be downloaded in parallel.

	fetchingMu.Lock()
	urlMut, ok := fetchingURLs[url]
	if !ok {
		urlMut = &sync.Mutex{}
		fetchingURLs[url] = urlMut
	}
	fetchingMu.Unlock()

	defer func() {
		fetchingMu.Lock()
		delete(fetchingURLs, url)
		fetchingMu.Unlock()
	}()

	urlMut.Lock()
	defer urlMut.Unlock()

	// Recheck with the acquired lock.
	if urlIsInvalid(url) {
		return nil, errURLNotFound
	}

	// Only acquire the semaphore once we've acquired the per-URL mutex, just to
	// ensure that all n different URLs can run in paralle.
	if err := parallel.Acquire(ctx, 1); err != nil {
		return nil, errors.Wrap(err, "failed to acquire ctx")
	}
	defer parallel.Release(1)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request %q", url)
	}

	client := httputil.FromContext(ctx, "img")

	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if r.StatusCode < 200 || r.StatusCode > 299 {
		if r.StatusCode >= 400 && r.StatusCode <= 499 {
			markURLInvalid(url)
		}

		r.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d getting %q", r.StatusCode, url)
	}

	return r.Body, nil
}
