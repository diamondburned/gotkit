package imgutil

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/httputil"
	"github.com/diamondburned/gotkit/internal/cachegc"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

var defaultClient = &http.Client{
	Timeout: 30 * time.Second,
}

// CacheAge is the age to keep for all cached images.
var CacheAge = 7 * 24 * time.Hour // 7 days cache

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

func fetchImage(ctx context.Context, url string, img ImageSetter, o Opts) error {
	if url == "" {
		return errors.New("empty URL given")
	}

	if urlIsInvalid(url) {
		return errURLNotFound
	}

	cacheDir := app.FromContext(ctx).CachePath("img2")
	cacheDst := urlPath(cacheDir, url)

	err := loadPixbufFromFile(ctx, cacheDst, img, o)
	cachegc.Do(cacheDir, CacheAge)

	if err == nil {
		return nil
	}

	if err := fetchURL(ctx, url, cacheDst); err == nil {
		return loadPixbufFromFile(ctx, cacheDst, img, o)
	}

	// See if this is a cache error. If it is, then just don't use the cache
	// at all.
	if cachegc.IsCacheError(err) {
		log.Println("cache error, falling back to HTTP:", err)

		r, err := getBody(ctx, url)
		if err != nil {
			return err
		}
		defer r.Close()

		return loadPixbuf(ctx, r, img, o)
	}

	// Otherwise, return.
	return err
}

func fetchURL(ctx context.Context, url, cacheDst string) error {
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
		return errURLNotFound
	}

	// Only acquire the semaphore once we've acquired the per-URL mutex, just to
	// ensure that all n different URLs can run in paralle.
	if err := parallel.Acquire(ctx, 1); err != nil {
		return errors.Wrap(err, "failed to acquire ctx")
	}
	defer parallel.Release(1)

	// Small time between the response being read and the file being created on
	// the disk, which might be an issue on slow computers, but whatever.
	err := cachegc.WithTmpFile(cacheDst, "*", func(f *os.File) error {
		return downloadTo(ctx, url, f)
	})

	if err != nil {
		return err
	}

	return nil
}

func downloadTo(ctx context.Context, url string, w io.Writer) error {
	r, err := getBody(ctx, url)
	if err != nil {
		return err
	}
	defer r.Close()

	if _, err := io.Copy(w, r); err != nil {
		return errors.Wrap(err, "cannot download")
	}

	return nil
}

func getBody(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request %q", url)
	}

	client := httputil.FromContext(ctx, defaultClient)

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

func urlPath(baseDir, url string) string {
	b := sha1.Sum([]byte(url))
	f := base64.URLEncoding.EncodeToString(b[:])
	return filepath.Join(baseDir, f)
}
