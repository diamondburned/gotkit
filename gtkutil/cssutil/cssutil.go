package cssutil

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

var globalCSS = func() *strings.Builder {
	b := strings.Builder{}
	b.Grow(50 << 10) // 50KB
	return &b
}()

// Applier returns a constructor that applies a class to the given widgetter. It
// also writes the CSS to the global CSS.
func Applier(class, css string) func(gtk.Widgetter) {
	WriteCSS(css)
	classes := strings.Split(class, ".")
	return func(w gtk.Widgetter) {
		for _, class := range classes {
			gtk.BaseWidget(w).AddCSSClass(class)
		}
	}
}

// Applyf is a convenient function that wraps Sprintf and Apply.
func Applyf(widget gtk.Widgetter, f string, v ...interface{}) {
	Apply(widget, fmt.Sprintf(f, v...))
}

// Apply applies the given CSS into the given widget's style context.
func Apply(widget gtk.Widgetter, css string) {
	prov := gtk.NewCSSProvider()
	prov.ConnectParsingError(func(sec *gtk.CSSSection, err error) {
		loc := sec.StartLocation()
		lines := strings.Split(css, "\n")
		log.Printf(
			"generated CSS error (%v) at line: %q\n%s",
			err, lines[loc.Lines()], debug.Stack(),
		)
	})
	prov.LoadFromData(css)

	w := gtk.BaseWidget(widget)
	s := w.StyleContext()
	s.AddProvider(prov, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

// WriteCSS adds the given string to the global CSS. It's primarily meant to be
// used during global variable initialization. If WriteCSS is called after
// ApplyGlobalCSS, then a panic is thrown.
func WriteCSS(css string) struct{} {
	globalCSS.WriteString(css)
	return struct{}{}
}

var _ = WriteCSS(`
	avatar { border-radius: 999px; }
`)

// AddClass adds classes.
func AddClass(w gtk.Widgetter, classes ...string) {
	ctx := gtk.BaseWidget(w).StyleContext()
	for _, class := range classes {
		ctx.AddClass(class)
	}
}

// ApplyGlobalCSS applies the current global CSS to the default display.
func ApplyGlobalCSS() {
	globalCSS := globalCSS.String()

	prov := gtk.NewCSSProvider()
	prov.ConnectParsingError(func(sec *gtk.CSSSection, err error) {
		loc := sec.StartLocation()

		lines := strings.Split(globalCSS, "\n")
		log.Printf("CSS error (%v) at line: %q", err, lines[loc.Lines()])
	})

	prov.LoadFromData(globalCSS)

	display := gdk.DisplayGetDefault()
	gtk.StyleContextAddProviderForDisplay(display, prov, 600) // app
}

// ApplyUserCSS applies the user CSS at the given path.
func ApplyUserCSS(path string) {
	f, err := os.ReadFile(path)
	if err != nil {
		log.Println("failed to read user.css:", err)
		return
	}

	if userCSS := string(f); userCSS != "" {
		prov := gtk.NewCSSProvider()
		prov.LoadFromData(userCSS)

		display := gdk.DisplayGetDefault()
		gtk.StyleContextAddProviderForDisplay(display, prov, 800) // user
	}
}
