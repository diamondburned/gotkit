// Package onlineimage contains image widgets that can fetch from image
// providers, usually online ones. It provides lazy HiDPI scaling by
// automatically reloading images when the scale factor changes.
package onlineimage

import (
	"context"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

type AnimationController baseImage

// Start starts the animation playback in the background. The animation isn't
// stopped until it is either unmapped or Stop is called.
func (c *AnimationController) Start() {
	(*baseImage)(c).startAnimation()
}

// Stop stops the animation playback.
func (c *AnimationController) Stop() {
	(*baseImage)(c).stopAnimation()
}

// OnHover binds the controller to a motion controller attached to the image
// widget. When the user hovers over the image, the animation plays.
func (c *AnimationController) OnHover() {
	c.ConnectMotion(c.imageParent)
}

// ConnectMotion connects a motion controller to the given widget that will
// activate the animation when it's hovered over (entered).
func (c *AnimationController) ConnectMotion(w gtk.Widgetter) {
	motion := gtk.NewEventControllerMotion()
	motion.ConnectEnter(func(x, y float64) { c.Start() })
	motion.ConnectLeave(func() { c.Stop() })

	base := gtk.BaseWidget(w)
	base.AddController(motion)
}

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

// SetSizeRequest sets the avatar size.
func (a *Avatar) SetSizeRequest(size int) {
	a.Avatar.SetSizeRequest(size)
	a.base.scaler.Invalidate()
}

// EnableAnimation enables animation for the avatar. The controller is returned
// for the user to determine when to play the animation.
func (a *Avatar) EnableAnimation() *AnimationController {
	return a.base.enableAnimation()
}

func (a *Avatar) set() imgutil.ImageSetter {
	return imgutil.ImageSetter{
		SetFromPixbuf:    a.SetFromPixbuf,
		SetFromPaintable: a.SetFromPaintable,
	}
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

// SetSizeRequest sets the minimum size of a widget.
func (i *Image) SetSizeRequest(w, h int) {
	i.Image.SetSizeRequest(w, h)
	i.base.scaler.Invalidate()
}

// EnableAnimation enables animation for the avatar. The controller is returned
// for the user to determine when to play the animation.
func (i *Image) EnableAnimation() *AnimationController {
	return i.base.enableAnimation()
}

func (i *Image) set() imgutil.ImageSetter {
	return imgutil.ImageSetter{
		SetFromPixbuf:    i.SetFromPixbuf,
		SetFromPaintable: i.SetFromPaintable,
	}
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

// SetSizeRequest sets the minimum size of a widget.
func (p *Picture) SetSizeRequest(w, h int) {
	p.Picture.SetSizeRequest(w, h)
	p.base.scaler.Invalidate()
}

// EnableAnimation enables animation for the avatar. The controller is returned
// for the user to determine when to play the animation.
func (p *Picture) EnableAnimation() *AnimationController {
	return p.base.enableAnimation()
}

func (p *Picture) set() imgutil.ImageSetter {
	return imgutil.ImageSetter{
		SetFromPixbuf:    p.SetPixbuf,
		SetFromPaintable: p.SetPaintable,
	}
}
