package imgutil

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/pkg/errors"
)

// Provider describes a universal resource provider.
type Provider interface {
	Schemes() []string
	Do(ctx context.Context, url *url.URL, f func(*gdkpixbuf.Pixbuf))
}

// DoProviderURL invokes a Provider with the given URI string (instead of a
// *url.URL instance).
func DoProviderURL(ctx context.Context, p Provider, uri string, f func(*gdkpixbuf.Pixbuf)) {
	url, err := url.Parse(uri)
	if err != nil {
		OptsError(ctx, err)
		return
	}

	p.Do(ctx, url, f)
}

// Providers holds multiple providers. A Providers instance is also a Provider
// in itself.
type Providers map[string]Provider

var _ Provider = Providers(nil)

// NewProviders creates a new Providers instance. Providers that are put last
// can override schemes of providers put before.
func NewProviders(providers ...Provider) Providers {
	m := make(Providers, len(providers))
	for _, prov := range providers {
		for _, scheme := range prov.Schemes() {
			m[scheme] = prov
		}
	}
	return m
}

// Schemes returns all schemes within the Providers. It exists only to implement
// Provider and generally shouldn't be used. The returned list is always sorted.
func (p Providers) Schemes() []string {
	schemes := make([]string, 0, len(p))
	for scheme := range p {
		schemes = append(schemes, scheme)
	}
	sort.Strings(schemes)
	return schemes
}

// Do invokes any of the providers inside.
func (p Providers) Do(ctx context.Context, url *url.URL, f func(*gdkpixbuf.Pixbuf)) {
	provider, ok := p[url.Scheme]
	if !ok {
		OptsError(ctx, fmt.Errorf("unknown scheme %q", url.Scheme))
		return
	}

	provider.Do(ctx, url, f)
}

type httpProvider struct{}

// HTTPProvider is the universal resource provider that handles HTTP and HTTPS
// schemes (http:// and https://).
var HTTPProvider Provider = httpProvider{}

// Schemes implements Provider.
func (p httpProvider) Schemes() []string {
	return []string{"http", "https"}
}

// Do implements Provider.
func (p httpProvider) Do(ctx context.Context, url *url.URL, f func(*gdkpixbuf.Pixbuf)) {
	AsyncGETPixbuf(ctx, url.String(), f)
}

type fileProvider struct{}

// FileProvider is the universal resource provider for a file (file://).
var FileProvider Provider = fileProvider{}

// Schemes implements Provider.
func (p fileProvider) Schemes() []string {
	return []string{"file"}
}

// Do implements Provider.
func (p fileProvider) Do(ctx context.Context, url *url.URL, f func(*gdkpixbuf.Pixbuf)) {
	go func() {
		o := OptsFromContext(ctx)

		var p *gdkpixbuf.Pixbuf
		var err error

		// Fast path with no size.
		if w, h := o.Size(); w == 0 && h == 0 {
			p, err = gdkpixbuf.NewPixbufFromFile(url.Path)
		} else {
			p, err = gdkpixbuf.NewPixbufFromFileAtScale(url.Path, w, h, true)
		}

		if err != nil {
			o.Error(errors.Wrap(err, "cannot create pixbuf"))
			return
		}

		glib.IdleAdd(func() {
			select {
			case <-ctx.Done():
				o.Error(ctx.Err())
			default:
				f(p)
			}
		})
	}()
}
