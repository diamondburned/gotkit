package onlineimage

import (
	"context"
	"net/url"

	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

// MaxFPS is the maximum FPS to play an animation (often a GIF) at. In reality,
// the actual frame rate heavily depends on the draw clock of GTK, but this
// duration determines the background ticker.
//
// For more information, see
// https://wunkolo.github.io/post/2020/02/buttery-smooth-10fps/.
const MaxFPS = 50

const maxFPSDelay = 1000 / MaxFPS

type imageParent interface {
	gtk.Widgetter
	set() imgutil.ImageSetter
}

type baseImage struct {
	imageParent
	prov imgutil.Provider

	scaler pixbufScaler

	animation *gdkpixbuf.PixbufAnimation
	animating glib.SourceHandle

	ctx gtkutil.Cancellable
	url string
	ok  bool

	animate bool
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

	imgutil.DoProviderURL(ctx, b.prov, url, imgutil.ImageSetter{
		SetFromPixbuf: func(p *gdkpixbuf.Pixbuf) {
			b.ok = true
			b.animation = nil
			b.scaler.SetFromPixbuf(p)
		},
		SetFromAnimation: func(anim *gdkpixbuf.PixbufAnimation) {
			b.ok = true
			b.scaler.SetFromPixbuf(anim.StaticImage())

			if b.animate {
				b.animation = anim
			} else {
				b.animation = nil
			}
		},
	})
}

func (b *baseImage) enableAnimation() *AnimationController {
	b.animate = true

	base := gtk.BaseWidget(b.imageParent)
	base.ConnectUnmap(b.stopAnimation)

	return (*AnimationController)(b)
}

func (b *baseImage) startAnimation() {
	if !b.animate || b.animating > 0 || b.animation == nil {
		return
	}

	iter := b.animation.Iter(nil)
	setter := b.imageParent.set()

	base := gtk.BaseWidget(b.imageParent)

	scale := base.ScaleFactor()
	if scale == 0 {
		return
	}

	w, h := b.size()
	w *= scale
	h *= scale

	var advance func()
	advance = func() {
		if iter.Advance(nil) {
			// Got new frame.
			p := iter.Pixbuf()
			// We only scale the pixbuf if our scale factor is 2x or 1x, because
			// 3x users likely won't notice a significance difference in
			// quality.
			if w > 0 && h > 0 && scale < 3 {
				// Scaling doesn't actually use that much more CPU
				// than not, but it depends on how big the image is.
				p = p.ScaleSimple(w, h, gdkpixbuf.InterpTiles)
			}
			setter.SetFromPixbuf(p)
		}

		if delay := animDelay(iter); delay != -1 {
			// Schedule next frame.
			b.animating = glib.TimeoutAddPriority(uint(delay), glib.PriorityLow, advance)
		} else {
			// End of animation.
			b.finishStopAnimation()
		}
	}
	// Kickstart the animation.
	advance()
}

func (b *baseImage) stopAnimation() {
	if b.animating != 0 {
		glib.SourceRemove(b.animating)
		b.animating = 0
		b.finishStopAnimation()
	}
}

func (b *baseImage) finishStopAnimation() {
	b.scaler.Invalidate()
}

func animDelay(iter *gdkpixbuf.PixbufAnimationIter) int {
	delayMs := iter.DelayTime()
	if delayMs == -1 {
		return -1
	}

	if delayMs < maxFPSDelay {
		delayMs = maxFPSDelay
	}

	return delayMs
}

func urlScheme(urlStr string) string {
	url, _ := url.Parse(urlStr)
	return url.Scheme
}
