package gtkutil

import (
	"log"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// BindRightClick binds the given widget to take in right-click gestures. The
// function will also check for long-hold gestures.
func BindRightClick(w gtk.Widgetter, f func()) {
	BindRightClickAt(w, func(x, y float64) { f() })
}

// BindRightClickAt is a version of BindRightClick with accurate coordinates
// given to the callback.
func BindRightClickAt(w gtk.Widgetter, f func(x, y float64)) {
	c := gtk.NewGestureClick()
	c.SetButton(3)       // secondary
	c.SetExclusive(true) // handle mouse only
	c.ConnectAfter("pressed", func(nPress int, x, y float64) {
		if nPress == 1 {
			f(x, y)
		}
	})

	l := gtk.NewGestureLongPress()
	l.SetTouchOnly(true)
	l.ConnectAfter("pressed", func(x, y float64) {
		f(x, y)
	})

	widget := gtk.BaseWidget(w)
	widget.AddController(c)
	widget.AddController(l)
}

// ForwardTyping forwards all typing events from w to dst.
func ForwardTyping(w, dst gtk.Widgetter) {
	ForwardTypingFunc(w, func() gtk.Widgetter { return dst })
}

func ForwardTypingFunc(w gtk.Widgetter, f func() gtk.Widgetter) {
	// Activator to focus on composer when typed on.
	typingHandler := gtk.NewEventControllerKey()
	// Run the handler at the last phase, after all key handlers have captured
	// the event.
	typingHandler.SetPropagationPhase(gtk.PhaseBubble)
	typingHandler.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if gdk.KeyvalToUnicode(keyval) == 0 {
			// Don't forward these.
			return false
		}

		dstWidget := f()
		if dstWidget == nil {
			return false
		}

		dst := gtk.BaseWidget(dstWidget)
		dst.GrabFocus()
		typingHandler.Forward(dst)
		return true
	})
	gtk.BaseWidget(w).AddController(typingHandler)
}

// AddCallbackShortcuts adds the given shortcuts to the widget. The shortcuts
// are given as a map of keybindings to callbacks.
func AddCallbackShortcuts(w gtk.Widgetter, shortcuts map[string]func()) {
	controller := gtk.NewShortcutController()

	for key, callback := range shortcuts {
		trigger := gtk.NewShortcutTriggerParseString(key)
		if trigger == nil {
			log.Panicf("gtkutil: failed to parse keybinding %q", key)
		}

		action := gtk.NewCallbackAction(func(gtk.Widgetter, *glib.Variant) bool {
			callback()
			return true
		})

		shortcut := gtk.NewShortcut(trigger, action)
		controller.AddShortcut(shortcut)
	}

	gtk.BaseWidget(w).AddController(controller)
}

// AddActionShortcuts adds the given shortcuts to the widget. The shortcuts are
// given as a map of keybindings to action names.
func AddActionShortcuts(w gtk.Widgetter, shortcuts map[string]string) {
	controller := gtk.NewShortcutController()

	for key, actionName := range shortcuts {
		trigger := gtk.NewShortcutTriggerParseString(key)
		if trigger == nil {
			log.Panicf("gtkutil: failed to parse keybinding %q", key)
		}

		action := gtk.NewNamedAction(actionName)
		shortcut := gtk.NewShortcut(trigger, action)
		controller.AddShortcut(shortcut)
	}

	gtk.BaseWidget(w).AddController(controller)
}
