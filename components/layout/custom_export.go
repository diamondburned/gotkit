package layout

// #include <gtk/gtk.h>
import "C"
import "github.com/diamondburned/gotk4/pkg/gtk/v4"

//export gotkit_layout_request_mode
func gotkit_layout_request_mode(_widget *C.GtkWidget) C.GtkSizeRequestMode {
	layoutManagerBox := layoutFromCWidget(_widget)
	widget := goWidgetFromCWidget(_widget)
	return C.GtkSizeRequestMode(layoutManagerBox.RequestMode(widget))
}

//export gotkit_layout_measure
func gotkit_layout_measure(
	_widget *C.GtkWidget,
	_orientation C.GtkOrientation,
	_forSize C.int,
	_minimum *C.int,
	_natural *C.int,
	_minimumBaseline *C.int,
	_naturalBaseline *C.int,
) {
	layoutManagerBox := layoutFromCWidget(_widget)
	widget := goWidgetFromCWidget(_widget)

	minimum, natural, minimumBaseline, naturalBaseline := layoutManagerBox.Measure(
		widget,
		gtk.Orientation(_orientation),
		int(_forSize),
	)

	*_minimum = C.int(minimum)
	*_natural = C.int(natural)
	*_minimumBaseline = C.int(minimumBaseline)
	*_naturalBaseline = C.int(naturalBaseline)
}

//export gotkit_layout_allocate
func gotkit_layout_allocate(
	_widget *C.GtkWidget,
	_width C.int,
	_height C.int,
	_baseline C.int,
) {
	layoutManagerBox := layoutFromCWidget(_widget)
	widget := goWidgetFromCWidget(_widget)

	layoutManagerBox.Allocate(
		widget,
		int(_width),
		int(_height),
		int(_baseline),
	)
}
