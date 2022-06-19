package cssutil

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"text/template"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

var globalCSS = func() *strings.Builder {
	b := strings.Builder{}
	b.Grow(50 << 10) // 50KB
	return &b
}()

var globalVariables = template.FuncMap{}

// AddCSSVariables adds the variables from the given map into the global
// variables map. This function must be called before ApplyGlobalCSS is called
// for it to have an effect.
//
// To use a variable, use the {$variable} syntax.
func AddCSSVariables(vars map[string]string) {
	for k, v := range vars {
		k := k
		v := v

		globalVariables[k] = func() string { return v }
	}
}

func templateCSS(name, css string) string {
	var err error

	t := template.New("")
	t.Delims("{$", "}")
	t.Funcs(globalVariables)

	t, err = t.Parse(globalCSS.String())
	if err != nil {
		log.Panicf("cannot parse CSS template %s: %v", name, err)
	}

	var tmplOutput strings.Builder
	if err := t.Execute(&tmplOutput, nil); err != nil {
		log.Panicf("cannot render CSS template %s: %v", name, err)
	}

	return tmplOutput.String()
}

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

// AddClass adds classes.
func AddClass(w gtk.Widgetter, classes ...string) {
	ctx := gtk.BaseWidget(w).StyleContext()
	for _, class := range classes {
		ctx.AddClass(class)
	}
}

// ApplyGlobalCSS applies the current global CSS to the default display.
func ApplyGlobalCSS() {
	globalCSS := templateCSS("global", globalCSS.String())

	prov := newCSSProvider(globalCSS)

	display := gdk.DisplayGetDefault()
	gtk.StyleContextAddProviderForDisplay(display, prov, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

// ApplyUserCSS applies the user CSS at the given path.
func ApplyUserCSS(path string) {
	f, err := os.ReadFile(path)
	if err != nil {
		log.Println("failed to read user.css:", err)
		return
	}

	if userCSS := string(f); userCSS != "" {
		userCSS = templateCSS("user.css", userCSS)

		prov := newCSSProvider(userCSS)

		display := gdk.DisplayGetDefault()
		// We use a higher priority than USER in order to override the user-
		// specific global CSS. This is fine, because this file is made by the
		// user anyway.
		gtk.StyleContextAddProviderForDisplay(display, prov, gtk.STYLE_PROVIDER_PRIORITY_USER+200)
	}
}

func newCSSProvider(css string) *gtk.CSSProvider {
	prov := gtk.NewCSSProvider()
	prov.ConnectParsingError(func(sec *gtk.CSSSection, err error) {
		loc := sec.StartLocation()

		lines := strings.Split(css, "\n")
		log.Printf("CSS error (%v) at line: %q", err, lines[loc.Lines()])
	})
	prov.LoadFromData(css)
	return prov
}
