package onlineimage

import (
	"context"
	"net/url"

	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

type imageParent interface {
	gtk.Widgetter
	setFromPixbuf(p *gdkpixbuf.Pixbuf)
}

type baseImage struct {
	imageParent
	prov imgutil.Provider

	scaler pixbufScaler
	ctx    gtkutil.Cancellable
	url    string
	ok     bool
}

// NewAvatar creates a new avatar.
func (b *baseImage) init(ctx context.Context, parent imageParent, p imgutil.Provider) {
	b.imageParent = parent
	b.prov = p
	b.scaler.init(b)

	b.ctx = gtkutil.WithVisibility(ctx, parent)
	b.ctx.OnRenew(func(ctx context.Context) func() {
		b.scaler.Invalidate()
		b.fetch(ctx)
		return nil
	})
}

func (b *baseImage) SetFromURL(url string) {
	if b.url == url {
		return
	}

	b.url = url
	b.refetch()
}

func (b *baseImage) refetch() {
	b.ok = false
	b.fetch(b.ctx.Take())
}

func (b *baseImage) size() (w, h int) {
	base := gtk.BaseWidget(b)

	w, h = base.SizeRequest()
	if w > 0 && h > 0 {
		return
	}

	rect := base.Allocation()
	w = rect.Width()
	h = rect.Height()

	return
}

func (b *baseImage) fetch(ctx context.Context) {
	if b.ok || ctx.Err() != nil {
		return
	}

	url := b.url
	if url == "" {
		b.scaler.SetFromPixbuf(nil)
		return
	}

	// Inject the rescaling size option.
	ctx = imgutil.WithOpts(ctx, imgutil.WithRescale(b.scaler.ParentSize()))

	imgutil.DoProviderURL(ctx, b.prov, url, func(p *gdkpixbuf.Pixbuf) {
		b.ok = true
		b.scaler.SetFromPixbuf(p)
	})
}

func urlScheme(urlStr string) string {
	url, _ := url.Parse(urlStr)
	return url.Scheme
}
