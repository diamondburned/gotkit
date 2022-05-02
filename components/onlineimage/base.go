package onlineimage

import (
	"context"
	"net/url"

	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
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

	scaler    pixbufScaler
	animation *animation

	ctx gtkutil.Cancellable
	url string
	ok  bool
}

type animation struct {
	pixbuf    *gdkpixbuf.PixbufAnimation
	animating glib.SourceHandle
	paused    bool
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
			b.scaler.SetFromPixbuf(p)

			if b.animation != nil {
				b.animation.pixbuf = nil
			}
		},
		SetFromAnimation: func(anim *gdkpixbuf.PixbufAnimation) {
			b.ok = true
			b.scaler.SetFromPixbuf(anim.StaticImage())

			if b.animation != nil {
				b.animation.pixbuf = anim
			}
		},
	})
}

func (b *baseImage) enableAnimation() *AnimationController {
	if !CanAnimate {
		return (*AnimationController)(b)
	}

	b.animation = &animation{}

	setPause := func(pause bool) {
		if pause {
			b.stopAnimation()
		}

		b.animation.paused = pause
	}

	base := gtk.BaseWidget(b.imageParent)
	base.ConnectMap(func() { setPause(false) })
	base.ConnectUnmap(func() { setPause(true) })

	var bindRoot func()
	var unbindRoot func()

	bindRoot = func() {
		if unbindRoot != nil {
			unbindRoot()
			unbindRoot = nil
		}

		w, ok := rootWindow(gtk.BaseWidget(b.imageParent).Root())
		if ok {
			s := w.NotifyProperty("is-active", func() {
				// Pause animation on window unfocus.
				setPause(!w.IsActive())
			})
			unbindRoot = func() { w.HandlerDisconnect(s) }
		}
	}

	b.NotifyProperty("root", bindRoot)
	bindRoot()

	return (*AnimationController)(b)
}

func rootWindow(w *gtk.Root) (*gtk.Window, bool) {
	if w == nil {
		return nil, false
	}

	obj := coreglib.InternObject(w)
	win := obj.WalkCast(func(obj glib.Objector) bool {
		_, isWindow := obj.(*gtk.Window)
		return isWindow
	})
	if win == nil {
		return nil, false
	}

	return win.(*gtk.Window), true
}

func (b *baseImage) startAnimation() {
	if b.animation == nil || b.animation.pixbuf == nil || b.animation.paused {
		return
	}

	iter := b.animation.pixbuf.Iter(nil)
	setter := b.imageParent.set()

	base := gtk.BaseWidget(b.imageParent)

	scale := base.ScaleFactor()
	if scale == 0 {
		return
	}

	w, h := b.size()
	w *= scale
	h *= scale

	useIter := func(iter *gdkpixbuf.PixbufAnimationIter) {
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
	// Kickstart the animation.
	useIter(iter)

	var scheduleNext func()
	scheduleNext = func() {
		if delay := animDelay(iter); delay != -1 {
			// Schedule next frame.
			b.animation.animating = glib.TimeoutAddPriority(uint(delay), glib.PriorityLow, func() {
				if iter.Advance(nil) {
					useIter(iter)
				}
				scheduleNext()
			})
		} else {
			// End of animation.
			b.stopAnimation()
		}
	}
	// Schedule the next frame.
	scheduleNext()
}

func (b *baseImage) stopAnimation() {
	if b.animation == nil {
		return
	}

	if b.animation.animating != 0 {
		glib.SourceRemove(b.animation.animating)
		b.animation.animating = 0
	}

	b.finishStopAnimation()
}

func (b *baseImage) finishStopAnimation() {
	if b.animation.pixbuf != nil {
		iter := b.animation.pixbuf.Iter(nil)
		b.scaler.SetFromPixbuf(iter.Pixbuf())
	} else {
		b.scaler.Invalidate()
	}
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
