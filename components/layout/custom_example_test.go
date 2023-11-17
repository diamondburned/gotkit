package layout_test

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/components/layout"
)

func ExampleCustomLayout() {
	layout := layout.New(layout.Funcs{
		RequestMode: func(w2 gtk.Widgetter) gtk.SizeRequestMode {
			return gtk.SizeRequestConstantSize
		},
		Measure: func(w2 gtk.Widgetter, orientation gtk.Orientation, forSize int) (
			minimum int,
			natural int,
			minimumBaseline int,
			naturalBaseline int,
		) {
			w := gtk.BaseWidget(w2)
			minimum, natural, minimumBaseline, naturalBaseline = w.Measure(orientation, forSize)
			if minimum < 100 {
				minimum = 100
			}
			return minimum, natural, minimumBaseline, naturalBaseline
		},
		Allocate: func(w2 gtk.Widgetter, width int, height int, baseline int) {
			w := gtk.BaseWidget(w2)
			w.Allocate(width, height, baseline, nil)
		},
	})

	label := gtk.NewLabel("Hello, world!")
	layout.SetForWidget(label)

	w := gtk.NewWindow()
	w.SetChild(label)
	w.Show()

	// Output:
}
