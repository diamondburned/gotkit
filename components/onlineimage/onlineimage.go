// Package onlineimage contains image widgets that can fetch from image
// providers, usually online ones. It provides lazy HiDPI scaling by
// automatically reloading images when the scale factor changes.
package onlineimage

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

var (
	_ imageParent = (*Avatar)(nil)
	_ imageParent = (*Image)(nil)
	_ imageParent = (*Picture)(nil)
)

// Avatar is an online variant of adaptive.Avatar.
type Avatar struct {
	*adaptive.Avatar
	base baseImage
}

// NewAvatar creates a new avatar.
func NewAvatar(ctx context.Context, p imgutil.Provider, size int) *Avatar {
	a := Avatar{Avatar: adaptive.NewAvatar(size)}
	a.AddCSSClass("onlineimage")
	a.base.init(ctx, &a, p)

	return &a
}

// SetFromURL sets the Avatar's URL.
func (a *Avatar) SetFromURL(url string) {
	a.base.SetFromURL(url)
}

func (a *Avatar) setFromPixbuf(p *gdkpixbuf.Pixbuf) {
	a.SetFromPixbuf(p)
}

// Image is an online variant of gtk.Image.
type Image struct {
	*gtk.Image
	base baseImage
}

// NewImage creates a new avatar.
func NewImage(ctx context.Context, p imgutil.Provider) *Image {
	i := Image{Image: gtk.NewImage()}
	i.AddCSSClass("onlineimage")
	i.base.init(ctx, &i, p)

	return &i
}

// SetFromURL sets the Image's URL.
func (i *Image) SetFromURL(url string) {
	i.base.SetFromURL(url)
}

func (i *Image) setFromPixbuf(p *gdkpixbuf.Pixbuf) {
	i.SetFromPixbuf(p)
}

// Picture is an online variant of gtk.Picture.
type Picture struct {
	*gtk.Picture
	base baseImage
}

// NewPicture creates a new Picture.
func NewPicture(ctx context.Context, prov imgutil.Provider) *Picture {
	p := Picture{Picture: gtk.NewPicture()}
	p.AddCSSClass("onlineimage")
	p.base.init(ctx, &p, prov)

	return &p
}

// SetURL sets the Avatar's URL.
func (p *Picture) SetURL(url string) {
	p.base.SetFromURL(url)
}

func (p *Picture) setFromPixbuf(pixbuf *gdkpixbuf.Pixbuf) {
	p.SetPixbuf(pixbuf)
}
