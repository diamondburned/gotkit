package autoscroll

import (
	"log/slog"
	"math"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type scrollState uint8

const (
	_ scrollState = iota
	bottomed
	locked
)

func (s scrollState) is(this scrollState) bool { return s == this }

// Window describes an automatically scrolled window.
type Window struct {
	*gtk.ScrolledWindow
	view   *gtk.Viewport
	vadj   *gtk.Adjustment
	logger *slog.Logger

	onBottomed func()

	upperValue   float64
	targetScroll float64
	state        scrollState
}

func NewWindow() *Window {
	w := Window{
		upperValue:   math.NaN(),
		targetScroll: math.NaN(),
		logger:       slog.Default().With("widget", "autoscroll.Window"),
	}

	w.ScrolledWindow = gtk.NewScrolledWindow()
	w.SetPropagateNaturalHeight(true)
	w.SetPlacement(gtk.CornerBottomLeft)

	w.vadj = w.ScrolledWindow.VAdjustment()

	w.view = gtk.NewViewport(nil, w.vadj)
	w.view.SetVScrollPolicy(gtk.ScrollNatural)
	w.view.SetScrollToFocus(false)
	w.SetChild(w.view)

	w.ConnectMap(func() {
		if !math.IsNaN(w.targetScroll) {
			w.scrollTo(w.targetScroll, false)
		}
	})

	var updatedValue bool

	w.vadj.ConnectChanged(func() {
		updatedValue = true
		w.upperValue = w.vadj.Upper()

		if w.state.is(bottomed) {
			// If the upper value changes and we're still bottomed, then we need
			// to scroll to the bottom again.
			newValue := w.upperValue - w.vadj.PageSize()
			w.logger.Debug(
				"upper value changed while bottomed, scrolling to bottom",
				"old_value", w.vadj.Value(),
				"new_value", newValue)

			w.scrollTo(newValue, true)
		}
	})

	w.vadj.ConnectValueChanged(func() {
		// Skip if we're locked, since we're only updating this if the state is
		// either bottomed or not.
		if w.state.is(locked) {
			return
		}

		// Check if the user has scrolled anywhere.
		bottomValue := w.upperValue - w.vadj.PageSize()
		if bottomValue < 0 || w.vadj.Value() >= bottomValue {
			w.logger.Debug(
				"user has scrolled to the bottom",
				"upper", w.upperValue,
				"value", w.vadj.Value(),
				"bottom_threshold", bottomValue)

			w.state = bottomed
			w.emitBottomed()
			return
		}

		// Either the user has scrolled somewhere else or GTK is still
		// trying to stabilize the layout. If the upper value does not
		// change in the next frame, then we can safely assume that the user
		// has scrolled somewhere else.
		updatedValue = false

		glib.IdleAdd(func() {
			if updatedValue {
				// The value changed, so GTK is still stabilizing the layout.
				return
			}

			w.logger.Debug(
				"user has scrolled somewhere else, unsetting bottomed state",
				"upper", w.upperValue,
				"value", w.vadj.Value(),
				"bottom_threshold", bottomValue)
			w.state = 0
		})
	})

	return &w
}

// Viewport returns the ScrolledWindow's Viewport.
func (w *Window) Viewport() *gtk.Viewport {
	return w.view
}

// VAdjustment overrides gtk.ScrolledWindow's.
func (w *Window) VAdjustment() *gtk.Adjustment {
	return w.vadj
}

// LockScroll locks the scroll to the current value, even if more content is
// added. The returned function unlocks the scroll.
func (w *Window) LockScroll() func() {
	w.state = locked

	old := getScrollAdjustments(w.vadj)
	w.logger.Debug(
		"scroll is now locked",
		"old_value", old.value)

	return func() {
		new := getScrollAdjustments(w.vadj)
		value := new.upper - old.upper + old.value

		w.logger.Debug(
			"scrolling to locked value",
			"old_upper", old.upper,
			"old_value", old.value,
			"new_upper", new.upper,
			"new_value", new.value,
			"scrolling_to_value", value)

		w.state = 0
		w.scrollTo(value, true)
	}
}

// Unbottom clears the bottomed state.
func (w *Window) Unbottom() {
	if w.state.is(bottomed) {
		w.state = 0
	}
}

// IsBottomed returns true if the scrolled window is currently bottomed out.
func (w *Window) IsBottomed() bool {
	return w.state.is(bottomed)
}

// ScrollToBottom scrolls the window to bottom.
func (w *Window) ScrollToBottom() {
	w.state = bottomed
	w.scrollTo(w.upperValue-w.vadj.PageSize(), false)
}

// OnBottomed registers the given function to be called when the user bottoms
// out the scrolled window.
func (w *Window) OnBottomed(f func()) {
	if w.onBottomed == nil {
		w.onBottomed = f
		return
	}

	old := w.onBottomed
	w.onBottomed = func() {
		old()
		f()
	}
}

func (w *Window) emitBottomed() {
	if w.onBottomed != nil {
		w.onBottomed()
	}
}

// SetChild sets the child of the ScrolledWindow.
func (w *Window) SetChild(child gtk.Widgetter) {
	_, scrollable := child.(gtk.Scrollabler)
	if scrollable {
		w.ScrolledWindow.SetChild(child)
	} else {
		w.view.SetChild(child)
		w.ScrolledWindow.SetChild(w.view)
	}
}

func (w *Window) scrollTo(targetScroll float64, deferFn bool) {
	w.targetScroll = targetScroll
	previousAdjs := getScrollAdjustments(w.vadj)

	doScroll := func() {
		if w.targetScroll != targetScroll {
			return
		}

		currentAdjs := getScrollAdjustments(w.vadj)
		if currentAdjs.upper != previousAdjs.upper {
			// Upper value changed while the layout was stabilizing.
			// Try to recalculate the target to accommodate for the new upper
			// value.
			targetScroll += currentAdjs.upper - previousAdjs.upper
		}

		w.logger.Debug(
			"emitting scroll event",
			"adj_previous", previousAdjs,
			"adj_current", currentAdjs,
			"wanted_target", w.targetScroll,
			"actual_target", targetScroll)

		w.vadj.SetValue(targetScroll)
	}

	if deferFn {
		// Schedule the scroll until next frame.
		glib.IdleAdd(doScroll)
	} else {
		doScroll()
	}
}

type scrollAdjustments struct {
	lower float64
	upper float64
	value float64
}

func getScrollAdjustments(adj *gtk.Adjustment) scrollAdjustments {
	return scrollAdjustments{
		lower: adj.Lower(),
		upper: adj.Upper(),
		value: adj.Value(),
	}
}
