package layout

// #cgo pkg-config: gtk4
// #include <gtk/gtk.h>
//
// extern GtkSizeRequestMode gotkit_layout_request_mode(GtkWidget *widget);
// extern void gotkit_layout_measure(GtkWidget *widget, GtkOrientation orientation, int for_size, int *minimum, int *natural, int *minimum_baseline, int *natural_baseline);
// extern void gotkit_layout_allocate(GtkWidget *widget, int width, int height, int baseline);
// extern void callbackDelete(gpointer data); // defined in gbox
//
// const GQuark gotkit_layout_quark() {
//   return g_quark_from_static_string("gotkit.layoutmanager");
// }
import "C"
import (
	"log"
	"unsafe"

	"github.com/diamondburned/gotk4/pkg/core/gbox"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// RequestModeFunc queries the widget for its preferred size request mode.
type RequestModeFunc func(w gtk.Widgetter) gtk.SizeRequestMode

// Measure is called to measure the size of the widget.
type MeasureFunc func(w gtk.Widgetter, orientation gtk.Orientation, forSize int) (
	minimum int,
	natural int,
	minimumBaseline int,
	naturalBaseline int)

// Allocate is called to allocate the size of the widget.
type Allocate func(w gtk.Widgetter, width int, height int, baseline int)

// CustomLayout is a custom layout manager.
type CustomLayout struct {
	layout *gtk.LayoutManager
	funcs  Funcs
}

// New creates a new custom layout manager.
func New(funcs Funcs) *CustomLayout {
	_layout := C.gtk_custom_layout_new(
		(*[0]byte)(C.gotkit_layout_request_mode),
		(*[0]byte)(C.gotkit_layout_measure),
		(*[0]byte)(C.gotkit_layout_allocate),
	)

	obj := coreglib.AssumeOwnership(unsafe.Pointer(_layout))

	layout := obj.WalkCast(func(obj coreglib.Objector) bool {
		_, ok := obj.(*gtk.LayoutManager)
		return ok
	})
	if layout == nil {
		panic("no marshaler for " + obj.TypeFromInstance().String() + " matching *gtk.LayoutManager")
	}

	return &CustomLayout{
		layout: layout.(*gtk.LayoutManager),
		funcs:  funcs,
	}
}

// SetForWidget sets the layout manager for the given widget.
// It handles the initialization of the layout manager for the widget.
func (l *CustomLayout) SetForWidget(w gtk.Widgetter) {
	l.initForWidget(w)
	gtk.BaseWidget(w).SetLayoutManager(l.layout)
}

// initForWidget initializes the layout manager for the given widget.
// This must be called when the widget's layout manager is set to the custom
// one, otherwise it will panic.
func (l *CustomLayout) initForWidget(w gtk.Widgetter) {
	quark := C.gotkit_layout_quark()
	goID := gbox.Assign(l.funcs)

	wptr := (*C.GObject)(unsafe.Pointer(coreglib.BaseObject(w).Native()))
	C.g_object_set_qdata_full(wptr, quark, C.gpointer(goID), (*[0]byte)(C.callbackDelete))
}

// Funcs is a box of functions for the layout manager.
type Funcs struct {
	RequestMode RequestModeFunc
	Measure     MeasureFunc
	Allocate    Allocate
}

func layoutFromCWidget(widget *C.GtkWidget) Funcs {
	quark := C.gotkit_layout_quark()
	goID := uintptr(C.g_object_get_qdata((*C.GObject)(unsafe.Pointer(widget)), quark))

	layoutManagerBox, ok := gbox.Get(goID).(Funcs)
	if !ok {
		log.Panicf(
			"widget %p (%s) has no layoutManagerBox, did you forget to call InitForWidget?",
			widget,
			// Accept this for now. We're panicking anyway.
			coreglib.Take(unsafe.Pointer(widget)).TypeFromInstance())
	}

	return layoutManagerBox
}

func goWidgetFromCWidget(widget *C.GtkWidget) gtk.Widgetter {
	// Copied from gotk4.

	objptr := unsafe.Pointer(widget)
	if objptr == nil {
		panic("object of type gtk.Widgetter is nil")
	}

	object := coreglib.Take(objptr)
	casted := object.WalkCast(func(obj coreglib.Objector) bool {
		_, ok := obj.(gtk.Widgetter)
		return ok
	})
	rv, ok := casted.(gtk.Widgetter)
	if !ok {
		panic("no marshaler for " + object.TypeFromInstance().String() + " matching gtk.Widgetter")
	}
	return rv
}
