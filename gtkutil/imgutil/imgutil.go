package imgutil

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gdkpixbuf/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/pkg/errors"
)

type ctxKey uint8

const (
	_ ctxKey = iota
	httpKey
	optsKey
)

type Opts struct {
	w, h  int
	setFn ImageSetter
	done  func(error)

	sizer struct {
		set interface {
			SetSizeRequest(w, h int)
			SizeRequest() (w, h int)
		}
		w, h int
	}
}

// OptsFromContext gets the Opts from the given context. If there is none, then
// a zero-value instance is returned.
func OptsFromContext(ctx context.Context) Opts {
	opts, _ := ctx.Value(optsKey).(Opts)
	return opts
}

// OptsError calls Error on the Opts inside the context. It is a convenient
// function.
func OptsError(ctx context.Context, err error) {
	o := OptsFromContext(ctx)
	o.Error(err)
}

func (o *Opts) processOpts(funcs []OptFunc) {
	for _, opt := range funcs {
		opt(o)
	}
}

func (o *Opts) needDone() {
	if o.done != nil {
		panic("o.done is already set")
	}
}

// ignoreErrors is the list of errors to not log.
var ignoreErrors = []error{
	context.Canceled,
}

func (o *Opts) onDone(err error) {
	if o.done != nil {
		done := o.done
		o.done = nil

		glib.IdleAdd(func() { done(err) })
		return
	}

	if err == nil {
		return
	}

	for _, ignore := range ignoreErrors {
		if errors.Is(err, ignore) {
			return
		}
	}

	log.Println("imgutil:", err)
}

// Error triggers the error handler inside OptFunc if there's one. Otherwise, an
// error is logged down. This is useful for asynchronous imgutil function
// wrappers to signal an error.
func (o *Opts) Error(err error) {
	if err != nil {
		o.onDone(err)
	}
}

// Size returns the requested size from the Opts or (0, 0) if there is none.
func (o *Opts) Size() (w, h int) {
	return o.w, o.h
}

// OptFunc is a type that can optionally modify the default internal options for
// each call.
type OptFunc func(*Opts)

// WithOpts injects the given imgutil.OptFunc options into the context. imgutil
// calls that takes in the returned context will have the given options. Calling
// WithOpts with a context returned from another WithOpts will make it create a
// copy that inherits the properties of the top-level Opts.
func WithOpts(ctx context.Context, optFuncs ...OptFunc) context.Context {
	opts, _ := ctx.Value(optsKey).(Opts)
	opts.processOpts(optFuncs)
	return context.WithValue(ctx, optsKey, opts)
}

// WithFallbackIcon makes image functions use the icon as the image given into
// the callback instead of a nil one. If name is empty, then dialog-error is
// used. Note that this function overrides WithErrorFn if it is after.
func WithFallbackIcon(name string) OptFunc {
	return func(o *Opts) {
		o.needDone()
		o.done = func(err error) {
			if err == nil || o.setFn.SetFromPaintable == nil {
				return
			}

			w, h := 24, 24
			if o.sizer.w != 0 {
				w = o.sizer.w
			}
			if o.sizer.h != 0 {
				h = o.sizer.h
			}

			icon := IconPaintable(name, w, h)
			o.setFn.SetFromPaintable(icon)
		}
	}
}

// IconPaintable gets the icon with the given name and returns the size. Nil is
// never returned.
func IconPaintable(name string, w, h int) gdk.Paintabler {
	if name == "" {
		name = "image-missing"
	}

	size := w
	if h < w {
		size = h
	}

	theme := gtk.IconThemeGetForDisplay(gdk.DisplayGetDefault())
	if theme == nil {
		panic("imgutil: cannot get IconTheme for default display")
	}

	return theme.LookupIcon(name, nil, size, gtkutil.ScaleFactor(), gtk.TextDirLTR, 0)
}

// WithErrorFn adds a callback that is called on an error.
func WithErrorFn(f func(error)) OptFunc {
	return func(o *Opts) {
		o.needDone()
		o.done = func(err error) {
			if err != nil {
				f(err)
			}
		}
	}
}

// WithDoneFn is like WithErrorFn, except it's called once the routine is done
// on the main thread with a possibly nil error.
func WithDoneFn(done func(error)) OptFunc {
	return func(o *Opts) {
		o.needDone()
		o.done = done
	}
}

// WithRectRescale is a convenient function around WithRescale for rectangular
// or circular images.
func WithRectRescale(size int) OptFunc {
	return WithRescale(size, size)
}

// WithRescale rescales the image to the given max width and height while
// respecting its aspect ratio. The given sizes will be used as the maximum
// sizes.
func WithRescale(w, h int) OptFunc {
	return func(o *Opts) { o.w, o.h = w, h }
}

// WithSizeOverrider overrides the widget's size request to be of the given
// size.
func WithSizeOverrider(widget gtk.Widgetter, w, h int) OptFunc {
	return func(o *Opts) {
		o.sizer.set = gtk.BaseWidget(widget)
		o.sizer.w = w
		o.sizer.h = h
	}
}

// AsyncGET GETs the given URL and calls f in the main loop. If the context is
// cancelled by the time GET is done, then f will not be called. If the given
// URL is nil, then the function does nothing.
//
// This function can be called from any thread. It will synchronize accordingly
// by itself.
func AsyncGET(ctx context.Context, url string, img ImageSetter) {
	get(ctx, url, img, true)
}

// GET gets the given URL into a Paintable.
func GET(ctx context.Context, url string, img ImageSetter) {
	get(ctx, url, img, false)
}

func get(ctx context.Context, url string, img ImageSetter, async bool) {
	o := OptsFromContext(ctx)
	o.setFn = img

	if url == "" {
		o.onDone(nil)
		return
	}

	fetch := func() {
		err := fetchImage(ctx, url, img, o)
		if err == nil {
			err = ctx.Err()
		}

		o.onDone(err)
	}

	if async {
		go fetch()
	} else {
		fetch()
	}
}

func loadPixbufFromFile(ctx context.Context, path string, img ImageSetter, o Opts) error {
	if o.w > 0 && o.h > 0 {
		// Slow path, since we need to use PixbufLoader to be able to rescale
		// this.
		f, err := os.Open(path)
		if err != nil {
			return errors.Wrap(err, "cannot open pixbuf file")
		}
		defer f.Close()

		return loadPixbuf(ctx, f, img, o)
	}

	anim, err := gdkpixbuf.NewPixbufAnimationFromFile(path)
	if err != nil {
		return errors.Wrap(err, "cannot create new pixbuf animation")
	}

	glib.IdleAdd(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if o.sizer.set != nil {
			maxW, maxH := o.sizer.w, o.sizer.h
			if maxW == 0 && maxH == 0 {
				maxW, maxH = o.sizer.set.SizeRequest()
			}
			if maxW == 0 && maxH == 0 {
				maxW, maxH = o.w, o.h
			}

			w := anim.Width()
			h := anim.Height()
			o.sizer.set.SetSizeRequest(MaxSize(w, h, maxW, maxH))
		}

		if img.SetFromAnimation != nil && !anim.IsStaticImage() {
			// Is actually a real animation. Call SetFromAnimation instead
			// of SetFromPixbuf to signify this.
			img.SetFromAnimation(anim)
			return
		}

		if img.SetFromPixbuf != nil {
			img.SetFromPixbuf(anim.StaticImage())
			return
		}

		if img.SetFromPaintable != nil {
			img.SetFromPaintable(gdk.NewTextureForPixbuf(anim.StaticImage()))
			return
		}
	})

	return nil
}

var errNilPixbuf = errors.New("nil pixbuf")

func loadPixbuf(ctx context.Context, r io.Reader, img ImageSetter, o Opts) error {
	var size [2]int

	loader := gdkpixbuf.NewPixbufLoader()
	loader.ConnectSizePrepared(func(w, h int) {
		if o.w > 0 && o.h > 0 {
			if w != o.w || h != o.h {
				w, h = MaxSize(w, h, o.w, o.h)
				loader.SetSize(w, h)
			}
		}

		if o.sizer.set != nil {
			size = [2]int{w, h}
		}
	})

	if err := pixbufLoaderReadFrom(loader, r); err != nil {
		return errors.Wrap(err, "reader error")
	}

	glib.IdleAdd(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if size != [2]int{} {
			maxW, maxH := o.sizer.w, o.sizer.h
			if maxW == 0 && maxH == 0 {
				maxW, maxH = o.sizer.set.SizeRequest()
			}
			if maxW == 0 && maxH == 0 {
				maxW, maxH = o.w, o.h
			}
			w, h := MaxSize(size[0], size[1], maxW, maxH)
			o.sizer.set.SetSizeRequest(w, h)
		}

		anim := loader.Animation()

		if img.SetFromAnimation != nil && !anim.IsStaticImage() {
			// Is actually a real animation. Call SetFromAnimation instead
			// of SetFromPixbuf to signify this.
			img.SetFromAnimation(anim)
			return
		}

		if img.SetFromPixbuf != nil {
			img.SetFromPixbuf(anim.StaticImage())
			return
		}

		if img.SetFromPaintable != nil {
			img.SetFromPaintable(gdk.NewTextureForPixbuf(anim.StaticImage()))
			return
		}
	})

	return nil
}

const defaultBufsz = 1 << 17 // 128KB

var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, defaultBufsz)
		return &b
	},
}

func pixbufLoaderReadFrom(l *gdkpixbuf.PixbufLoader, r io.Reader) error {
	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)

	_, err := io.CopyBuffer(gioutil.PixbufLoaderWriter(l), r, *buf)
	if err != nil {
		l.Close()
		return err
	}

	if err := l.Close(); err != nil {
		return fmt.Errorf("failed to close PixbufLoader: %w", err)
	}

	return nil
}

// MaxSize returns the maximum size that can fit within the given max width and
// height. Aspect ratio is preserved.
func MaxSize(w, h, maxW, maxH int) (int, int) {
	if w == 0 {
		w = maxW
	}
	if h == 0 {
		h = maxH
	}
	if w < maxW && h < maxH {
		return w, h
	}

	wf := float64(w)
	hf := float64(h)

	scale := math.Min(
		float64(maxW)/wf,
		float64(maxH)/hf,
	)

	w = int(math.Round(wf * scale))
	h = int(math.Round(hf * scale))

	return w, h
}
