package imgutil

import (
	"context"
	"fmt"
	"io"
	"log"
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
	setFn interface{}
	err   func(error)

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

func (o *Opts) error(err error, writeLog bool) {
	if o.err != nil {
		gtkutil.InvokeMain(func() { o.err(err) })
	} else if writeLog {
		log.Println("imgutil:", err)
	}
}

// ignoreErrors is the list of errors to not log.
var ignoreErrors = []error{
	context.Canceled,
}

// Error triggers the error handler inside OptFunc if there's one. Otherwise, an
// error is logged down. This is useful for asynchronous imgutil function
// wrappers to signal an error.
func (o *Opts) Error(err error) {
	log := true
	for _, ignore := range ignoreErrors {
		if errors.Is(err, ignore) {
			log = false
			break
		}
	}
	o.error(err, log)
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
//
// This function only works with AsyncRead and AsyncGET. Using this elsewhere
// will result in a panic.
func WithFallbackIcon(name string) OptFunc {
	return func(o *Opts) {
		o.err = func(error) {
			fn, ok := o.setFn.(func(gdk.Paintabler))
			if !ok {
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
			fn(icon)
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
	return func(o *Opts) { o.err = f }
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

// AsyncRead reads the given reader asynchronously into a paintable.
func AsyncRead(ctx context.Context, r io.ReadCloser, f func(gdk.Paintabler)) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		r.Close()
	}()

	o := OptsFromContext(ctx)
	o.setFn = f

	do(ctx, o, true, func() (func(), error) {
		defer cancel()

		var paintable gdk.Paintabler

		p, err := readPixbuf(r, o)
		if err == nil {
			paintable = gdk.NewTextureForPixbuf(p)
		}

		return func() { f(paintable) }, nil
	})
}

// Read synchronously reads the reader into a paintable. The given context does
// NOT interrupt the read once cancelled.
func Read(ctx context.Context, r io.Reader) (gdk.Paintabler, error) {
	var paintable gdk.Paintabler
	var err error

	o := OptsFromContext(ctx)
	o.setFn = func(p gdk.Paintabler) { paintable = p }

	p, err := readPixbuf(r, o)
	if err == nil {
		paintable = gdk.NewTextureForPixbuf(p)
	}

	return paintable, err
}

// AsyncGET GETs the given URL and calls f in the main loop. If the context is
// cancelled by the time GET is done, then f will not be called. If the given
// URL is nil, then the function does nothing.
//
// This function can be called from any thread. It will synchronize accordingly
// by itself.
func AsyncGET(ctx context.Context, url string, f func(gdk.Paintabler)) {
	if url == "" {
		return
	}

	o := OptsFromContext(ctx)
	o.setFn = f

	do(ctx, o, true, func() (func(), error) {
		p, err := get(ctx, url, o)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
}

// GET gets the given URL into a Paintable.
func GET(ctx context.Context, url string, f func(gdk.Paintabler)) {
	if url == "" {
		return
	}

	o := OptsFromContext(ctx)
	o.setFn = f

	do(ctx, o, false, func() (func(), error) {
		p, err := get(ctx, url, o)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
}

func get(ctx context.Context, url string, o Opts) (gdk.Paintabler, error) {
	pixbuf, err := getPixbuf(ctx, url, o)
	if err != nil {
		return nil, err
	}

	return gdk.NewTextureForPixbuf(pixbuf), nil
}

// AsyncGETPixbuf fetches a pixbuf.
func AsyncGETPixbuf(ctx context.Context, url string, f func(*gdkpixbuf.Pixbuf)) {
	if url == "" {
		return
	}

	o := OptsFromContext(ctx)

	do(ctx, o, true, func() (func(), error) {
		p, err := getPixbuf(ctx, url, o)
		if err != nil {
			return nil, err
		}

		return func() { f(p) }, nil
	})
}

// GETPixbuf gets the Pixbuf directly.
func GETPixbuf(ctx context.Context, url string) (*gdkpixbuf.Pixbuf, error) {
	return getPixbuf(ctx, url, OptsFromContext(ctx))
}

func getPixbuf(ctx context.Context, url string, o Opts) (*gdkpixbuf.Pixbuf, error) {
	if url == "" {
		return nil, errors.New("empty URL given")
	}

	r, err := fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	p, err := readPixbuf(r, o)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %q", url)
	}

	return p, nil
}

func do(ctx context.Context, o Opts, async bool, do func() (func(), error)) {
	if async {
		go doImpl(ctx, o, do)
	} else {
		doImpl(ctx, o, do)
	}
}

func doImpl(ctx context.Context, o Opts, do func() (func(), error)) {
	f, err := do()
	if err != nil {
		o.error(err, true)
		return
	}

	glib.IdleAdd(func() {
		select {
		case <-ctx.Done():
			// don't call f if cancelledd
			o.error(ctx.Err(), false)
		default:
			f()
		}
	})
}

var errNilPixbuf = errors.New("nil pixbuf")

func readPixbuf(r io.Reader, o Opts) (*gdkpixbuf.Pixbuf, error) {
	loader := gdkpixbuf.NewPixbufLoader()
	loader.ConnectSizePrepared(func(w, h int) {
		if o.w > 0 && o.h > 0 {
			if w != o.w || h != o.h {
				w, h = MaxSize(w, h, o.w, o.h)
				loader.SetSize(w, h)
			}
		}
		if o.sizer.set != nil {
			maxW, maxH := o.sizer.w, o.sizer.h
			if maxW == 0 && maxH == 0 {
				maxW, maxH = o.sizer.set.SizeRequest()
			}
			if maxW == 0 && maxH == 0 {
				maxW, maxH = o.w, o.h
			}
			o.sizer.set.SetSizeRequest(MaxSize(w, h, maxW, maxH))
		}
	})

	if err := pixbufLoaderReadFrom(loader, r); err != nil {
		return nil, errors.Wrap(err, "reader error")
	}

	pixbuf := loader.Pixbuf()
	if pixbuf == nil {
		return nil, errNilPixbuf
	}

	return pixbuf, nil
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

	if w > h {
		h = h * maxW / w
		w = maxW
	} else {
		w = w * maxH / h
		h = maxH
	}

	return w, h
}
