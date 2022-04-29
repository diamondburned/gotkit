package imgutil

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Provider describes a universal resource provider.
type Provider interface {
	Schemes() []string
	Do(ctx context.Context, url *url.URL, img ImageSetter)
}

// ImageSetter contains functions for setting images fetched from a Provider.
type ImageSetter struct {
	SetFromPixbuf    func(*gdkpixbuf.Pixbuf)
	SetFromAnimation func(*gdkpixbuf.PixbufAnimation)
	SetFromPaintable func(gdk.Paintabler)
}

// ImageSetterFromImage returns an ImageSetter for a gtk.Image.
func ImageSetterFromImage(img *gtk.Image) ImageSetter {
	return ImageSetter{
		SetFromPaintable: img.SetFromPaintable,
		SetFromPixbuf:    img.SetFromPixbuf,
	}
}

// ImageSetterFromPicture returns an ImageSetter for a gtk.Picture.
func ImageSetterFromPicture(picture *gtk.Picture) ImageSetter {
	return ImageSetter{
		SetFromPaintable: picture.SetPaintable,
		SetFromPixbuf:    picture.SetPixbuf,
	}
}

// DoProviderURL invokes a Provider with the given URI string (instead of a
// *url.URL instance).
func DoProviderURL(ctx context.Context, p Provider, uri string, img ImageSetter) {
	url, err := url.Parse(uri)
	if err != nil {
		OptsError(ctx, err)
		return
	}

	p.Do(ctx, url, img)
}

const sizeFragmentf = "%dx%d"

// AppendURLSize appends into the URL fragments the width and height parameters.
// Providers that support these fragments will be able to read them.
func AppendURLSize(urlstr string, w, h int) string {
	u, err := url.Parse(urlstr)
	if err != nil {
		return urlstr
	}
	u.Fragment = fmt.Sprintf(sizeFragmentf, w, h)
	return u.String()
}

// ParseURLSize parses the optional width and height fragments from the URL. If
// the URL has none, then (0, 0) is returned.
func ParseURLSize(url *url.URL) (w, h int) {
	fmt.Sscanf(url.Fragment, sizeFragmentf, &w, &h)
	return
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
func (p Providers) Do(ctx context.Context, url *url.URL, img ImageSetter) {
	provider, ok := p[url.Scheme]
	if !ok {
		OptsError(ctx, fmt.Errorf("unknown scheme %q", url.Scheme))
		return
	}

	provider.Do(ctx, url, img)
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
func (p httpProvider) Do(ctx context.Context, url *url.URL, img ImageSetter) {
	AsyncGET(ctx, url.String(), img)
}

type fileProvider struct{}

// FileProvider is the universal resource provider for a file (file://).
var FileProvider Provider = fileProvider{}

// Schemes implements Provider.
func (p fileProvider) Schemes() []string {
	return []string{"file"}
}

// Do implements Provider.
func (p fileProvider) Do(ctx context.Context, url *url.URL, img ImageSetter) {
	go func() {
		o := OptsFromContext(ctx)
		if err := loadPixbufFromFile(ctx, url.Host+url.Path, img, o); err != nil {
			o.Error(err)
		}
	}()
}
