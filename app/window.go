package app

import (
	"context"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/errpopup"
	"github.com/diamondburned/gotkit/gtkutil"
)

// Window wraps a gtk.ApplicationWindow.
type Window struct {
	gtk.Window
	app *Application
}

// NewWindow creates a new Window bounded to the Application instance.
func NewWindow(app *Application) *Window {
	window := gtk.NewApplicationWindow(app.Application)
	window.SetDefaultSize(600, 400)
	return WrapWindow(app, window)
}

// WrapWindow wraps a [gtk.ApplicationWindow] into an app.Window.
func WrapWindow(app *Application, window *gtk.ApplicationWindow) *Window {
	gtkutil.ScaleFactor()

	window.SetApplication(app.Application)
	if isDevel {
		window.AddCSSClass("devel")
	}

	w := Window{
		Window: window.Window,
		app:    app,
	}
	w.SetLoading()

	return &w
}

// WithWindow injects the given Window instance into a context. The returned
// context will be cancelled if the window is closed.
func WithWindow(ctx context.Context, win *Window) context.Context {
	ctx = context.WithValue(ctx, windowKey, win)

	ctx, cancel := context.WithCancel(ctx)
	win.ConnectDestroy(cancel)

	return ctx
}

// WindowFromContext returns the context's window.
func WindowFromContext(ctx context.Context) *Window {
	win, _ := ctx.Value(windowKey).(*Window)
	return win
}

// GTKWindowFromContext returns the context's window. If the context does not
// have a window, then the active window is returned.
func GTKWindowFromContext(ctx context.Context) *gtk.Window {
	win, _ := ctx.Value(windowKey).(*Window)
	if win != nil {
		return &win.Window
	}

	app := FromContext(ctx)
	return app.ActiveWindow()
}

// SetTitle sets the main window's title.
func SetTitle(ctx context.Context, title string) {
	WindowFromContext(ctx).SetTitle(title)
}

// OpenURI opens the given URI using the system's default application.
func OpenURI(ctx context.Context, uri string) {
	if uri == "" {
		return
	}
	ts := uint32(time.Now().Unix())
	gtk.ShowURI(GTKWindowFromContext(ctx), uri, ts)
}

// Application returns the Window's parent Application instance.
func (w *Window) Application() *Application { return w.app }

// Error shows an error popup.
func (w *Window) Error(err ...error) {
	errpopup.Show(&w.Window, filterAndLogErrors("error:", err), func() {})
}

// Fatal shows a fatal error popup and closes the window afterwards.
func (w *Window) Fatal(err ...error) {
	errpopup.Fatal(&w.Window, filterAndLogErrors("fatal:", err)...)
}

// SetLoading shows a spinning circle. It disables the window.
func (w *Window) SetLoading() {
	spinner := gtk.NewSpinner()
	spinner.SetSizeRequest(24, 24)
	spinner.SetHAlign(gtk.AlignCenter)
	spinner.SetVAlign(gtk.AlignCenter)
	spinner.Start()

	w.Window.SetChild(spinner)
	w.SetTitle("Loading")
	w.NotifyChild(true, func() { spinner.Stop() })
}

// NotifyChild calls f if the main window's child is changed. If once is true,
// then f is never called again.
func (w *Window) NotifyChild(once bool, f func()) {
	var childHandle glib.SignalHandle
	childHandle = w.Window.NotifyProperty("child", func() {
		f()
		if once {
			w.Window.HandlerDisconnect(childHandle)
		}
	})
}

// SetSensitive sets whether or not the application's window is enabled.
func (w *Window) SetSensitive(sensitive bool) {
	w.Window.SetSensitive(sensitive)
}

// NewHeader creates a new header and puts it into the application window.
func (w *Window) NewHeader() *gtk.HeaderBar {
	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(true)
	w.Window.SetTitlebar(header)

	return header
}

// NewWindowHeader creates a new blank header.
func (w *Window) NewWindowHandle() *gtk.WindowHandle {
	header := gtk.NewWindowHandle()
	w.Window.SetTitlebar(header)

	return header
}

// SetTitle sets the application (and the main instance window)'s title.
func (w *Window) SetTitle(title string) {
	w.Window.SetTitle(w.app.SuffixedTitle(title))
}
