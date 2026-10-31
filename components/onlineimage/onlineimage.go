// Package onlineimage contains a gdk.Paintable that draws images fetched from
// image providers, usually online ones. The paintable can be shown by any
// widget that takes a paintable, such as gtk.Picture and gtk.Image.
package onlineimage

import (
	"context"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// MaxFPS is the maximum FPS to play an animation (often a GIF) at.
//
// For more information, see
// https://wunkolo.github.io/post/2020/02/buttery-smooth-10fps/.
const MaxFPS = 50

const maxFPSDelay = 1000 / MaxFPS

// Paintable is a gdk.Paintable that draws an image fetched from an
// imgutil.Provider. The image is only fetched once the paintable is first
// drawn, so images that are never shown are never fetched. GTK scales the image
// when drawing it, so it stays sharp at any size and scale factor.
type Paintable struct {
	coreglib.Object
	*gdk.Paintable

	ctx  context.Context
	prov imgutil.Provider
	url  string

	fetched bool
	cancel  context.CancelFunc
	// eager is true if the image is fetched without being drawn first.
	eager bool

	// image is the fetched image, or nil if there is none yet.
	image   gdk.Paintabler
	anim    *animation
	playing bool
}

// animation is an animated image. Its frames are drawn while it is playing.
type animation struct {
	pixbuf *gdkpixbuf.PixbufAnimation
	iter   *gdkpixbuf.PixbufAnimationIter
	frame  *gdk.Texture
	// next draws the next frame.
	next glib.SourceHandle
}

var paintableType = coreglib.RegisterSubclass[*Paintable](
	coreglib.WithGTypeName("GotkitOnlinePaintable"),
	coreglib.WithOverrides(func(p *Paintable) coreglib.ObjectOverrides {
		return coreglib.ObjectOverrides{
			Init:    func() { p.Paintable = &gdk.Paintable{Object: &p.Object} },
			Dispose: p.reset,
		}
	}),
	coreglib.WithOverrides(func(p *Paintable) gdk.PaintableOverrides {
		return gdk.PaintableOverrides{
			IntrinsicWidth:  p.intrinsicWidth,
			IntrinsicHeight: p.intrinsicHeight,
			Snapshot:        p.snapshot,
		}
	}),
)

// NewPaintable creates a new Paintable that fetches images using the given
// provider. imgutil options in ctx apply to every fetched image.
func NewPaintable(ctx context.Context, prov imgutil.Provider) *Paintable {
	p := paintableType.New()
	p.ctx = ctx
	p.prov = prov
	return p
}

// URL returns the URL of the image.
func (p *Paintable) URL() string {
	return p.url
}

// SetURL sets the URL of the image. An empty URL clears the image.
func (p *Paintable) SetURL(url string) {
	if p.url == url {
		return
	}

	hadImage := p.image != nil

	p.reset()
	p.url = url

	if hadImage {
		p.InvalidateSize()
	}
	// Redraw, which fetches the new image if the paintable is shown.
	p.InvalidateContents()
	if p.eager {
		p.Fetch()
	}
}

// HasImage returns true if the image has been fetched.
func (p *Paintable) HasImage() bool {
	return p.image != nil
}

// Fetch fetches the image if it has not been fetched yet. Drawing the paintable
// fetches the image, so this is only needed when the image must be fetched
// before the paintable is drawn.
func (p *Paintable) Fetch() {
	if p.fetched || p.url == "" {
		return
	}
	p.fetched = true

	ctx, cancel := context.WithCancel(p.ctx)
	p.cancel = cancel

	imgutil.DoProviderURL(ctx, p.prov, p.url, imgutil.ImageSetter{
		SetFromPixbuf: func(pixbuf *gdkpixbuf.Pixbuf) {
			p.setImage(ctx, gdk.NewTextureForPixbuf(pixbuf), nil)
		},
		SetFromAnimation: func(anim *gdkpixbuf.PixbufAnimation) {
			p.setImage(ctx, gdk.NewTextureForPixbuf(anim.StaticImage()), anim)
		},
		SetFromPaintable: func(image gdk.Paintabler) {
			p.setImage(ctx, image, nil)
		},
	})
}

func (p *Paintable) setImage(ctx context.Context, image gdk.Paintabler, anim *gdkpixbuf.PixbufAnimation) {
	// The URL was changed or the paintable was disposed.
	if ctx.Err() != nil {
		return
	}

	p.image = image
	if anim != nil {
		p.anim = &animation{pixbuf: anim}
	}

	p.InvalidateSize()
	p.InvalidateContents()
}

// reset cancels fetching and drops the image.
func (p *Paintable) reset() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	p.stopAnimation()
	p.fetched = false
	p.image = nil
	p.anim = nil
}

// SetPlaying sets whether the animation of an animated image plays. It only
// plays while the paintable is being drawn.
func (p *Paintable) SetPlaying(playing bool) {
	if p.playing == playing {
		return
	}

	p.playing = playing
	if !playing {
		p.stopAnimation()
	}

	if p.anim != nil {
		p.InvalidateContents()
	}
}

// FetchOnMap fetches the image while the given widget is mapped instead of
// waiting for the paintable to be drawn. GTK never draws a widget without a
// size, so widgets that are only sized by the image need this.
func (p *Paintable) FetchOnMap(widget gtk.Widgetter) {
	w := gtk.BaseWidget(widget)
	w.ConnectMap(func() {
		p.eager = true
		p.Fetch()
	})
	w.ConnectUnmap(func() { p.eager = false })
	if w.Mapped() {
		p.eager = true
		p.Fetch()
	}
}

// PlayOnHover plays the animation while the pointer is over the given widget.
func (p *Paintable) PlayOnHover(widget gtk.Widgetter) {
	motion := gtk.NewEventControllerMotion()
	motion.ConnectEnter(func(x, y float64) { p.SetPlaying(true) })
	motion.ConnectLeave(func() { p.SetPlaying(false) })
	gtk.BaseWidget(widget).AddController(motion)
}

func (p *Paintable) stopAnimation() {
	if p.anim == nil {
		return
	}

	if p.anim.next != 0 {
		glib.SourceRemove(p.anim.next)
		p.anim.next = 0
	}

	p.anim.iter = nil
	p.anim.frame = nil
}

// currentFrame returns the image to draw, advancing the animation if it is
// playing.
func (p *Paintable) currentFrame() gdk.Paintabler {
	anim := p.anim
	if anim == nil || !p.playing {
		return p.image
	}

	if anim.iter == nil {
		anim.iter = anim.pixbuf.Iter(nil)
		anim.frame = gdk.NewTextureForPixbuf(anim.iter.Pixbuf())
	} else if anim.iter.Advance(nil) {
		anim.frame = gdk.NewTextureForPixbuf(anim.iter.Pixbuf())
	}

	// Only draw the next frame after this one is drawn, so the animation stops
	// once the paintable is no longer shown.
	if delay := anim.iter.DelayTime(); delay >= 0 && anim.next == 0 {
		anim.next = glib.TimeoutAdd(uint(max(delay, maxFPSDelay)), func() {
			anim.next = 0
			p.InvalidateContents()
		})
	}

	return anim.frame
}

func (p *Paintable) intrinsicWidth() int {
	if p.image == nil {
		return 0
	}
	return p.image.IntrinsicWidth()
}

func (p *Paintable) intrinsicHeight() int {
	if p.image == nil {
		return 0
	}
	return p.image.IntrinsicHeight()
}

func (p *Paintable) snapshot(snapshot gdk.Snapshotter, width, height float64) {
	p.Fetch()

	image := p.currentFrame()
	if image == nil {
		return
	}

	// GTK picks the scaling filter itself. Forcing trilinear filtering blurs
	// HiDPI images, since it mixes in a half-size mipmap whenever the image is
	// less than twice the drawn size.
	image.Snapshot(snapshot, width, height)
}

// Avatar is an adw.Avatar that shows the image of a Paintable. adw.Avatar hides
// its initials as soon as it has a custom image, so the paintable only becomes
// the custom image once its image is fetched.
type Avatar struct {
	*adw.Avatar
	Paintable *Paintable
}

// NewAvatar creates a new Avatar.
func NewAvatar(ctx context.Context, prov imgutil.Provider, size int) *Avatar {
	a := &Avatar{
		Avatar:    adw.NewAvatar(size, "", true),
		Paintable: NewPaintable(ctx, prov),
	}
	a.AddCSSClass("onlineimage")

	// The avatar never draws the paintable before it is the custom image.
	a.Paintable.FetchOnMap(a.Avatar)

	// The paintable must not hold a strong reference to the avatar, which
	// holds the paintable.
	avatar := coreglib.NewWeakRef(a.Avatar)
	paintable := a.Paintable
	paintable.ConnectInvalidateSize(func() {
		a := avatar.Get()
		if a == nil {
			return
		}
		if paintable.HasImage() {
			a.SetCustomImage(paintable)
		} else {
			a.SetCustomImage(nil)
		}
	})

	return a
}

// SetFromURL sets the URL of the avatar's image.
func (a *Avatar) SetFromURL(url string) {
	a.Paintable.SetURL(url)
}
