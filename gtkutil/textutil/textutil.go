// Package textutil contains utilities for handling Pango markup and
// TextBuffer shenanigans.
package textutil

import (
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"html"
	"image/color"
	"log"
	"math"
	"strings"
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
)

// Attrs is a way to declaratively create a pango.AttrList.
func Attrs(attrs ...*pango.Attribute) *pango.AttrList {
	list := pango.NewAttrList()
	for _, attr := range attrs {
		list.Insert(attr)
	}
	return list
}

// NewAttrOpacity creates a new AttrForegroundAlpha.
func NewAttrOpacity(alpha float64) *pango.Attribute {
	if alpha > 1 || alpha < 0 {
		panic("alpha out of bounds [0.0, 1.0]")
	}
	return pango.NewAttrForegroundAlpha(uint16(math.Round(alpha * 0xFFFF)))
}

// ErrorMarkup formats the given message red using Pango markup.
func ErrorMarkup(msg string) string {
	msg = strings.TrimPrefix(msg, "error ")
	return fmt.Sprintf(
		`<span color="#FF0033"><b>Error!</b></span> %s`,
		html.EscapeString(msg),
	)
}

var errorAttrs = Attrs(
	pango.NewAttrInsertHyphens(false),
)

// ErrorLabel makes a new label with the class `.error'.
func ErrorLabel(markup string) *gtk.Label {
	errLabel := gtk.NewLabel(markup)
	errLabel.SetUseMarkup(true)
	errLabel.SetSelectable(true)
	errLabel.SetWrap(true)
	errLabel.SetWrapMode(pango.WrapWordChar)
	errLabel.SetCSSClasses([]string{"error"})
	errLabel.SetAttributes(errorAttrs)
	return errLabel
}

// TabWidth is the width of a tab character in regular monospace characters.
var TabWidth = prefs.NewInt(4, prefs.IntMeta{
	Name:        "Tab Width",
	Section:     "Text",
	Description: "The tab width in characters.",
	Min:         0,
	Max:         16,
})

var monospaceAttr = Attrs(
	pango.NewAttrFamily("Monospace"),
)

// SetTabSize sets the given TextView's tab size.
func SetTabSize(text *gtk.TextView) {
	layout := text.CreatePangoLayout(" ")
	layout.SetAttributes(monospaceAttr)

	width, _ := layout.PixelSize()

	stops := pango.NewTabArray(1, true)
	stops.SetTab(0, pango.TabLeft, TabWidth.Value()*width)

	text.SetTabs(stops)
}

// RGBHex converts the given color to a HTML hex color string. The alpha value
// is ignored.
func RGBHex(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// cachedLinkTags is cached for the duration of a single event loop.
var cachedLinkTags TextTagsMap

// LinkTags gets the text tags with colors for a, a:hover and a:visited. The
// output of the function is cached for a short while, so the user doesn't have
// to store it. It is concurrently safe to call this function.
func LinkTags() TextTagsMap {
	var tags TextTagsMap

	gtkutil.InvokeMain(func() {
		if cachedLinkTags != nil {
			tags = cachedLinkTags
			return
		}

		linkButton := gtk.NewLinkButton("")
		s := linkButton.StyleContext()

		m := make(TextTagsMap, 3)
		m.SetTagAttr("a", "insert-hyphens", false)

		s.SetState(gtk.StateFlagLink)
		link := s.Color()
		// 85% opacity unhovered; 100% opacity hovered.
		m.SetTagAttr("a", "foreground", rgbHex(link)+"CC")
		m.SetTagAttr("a:hover", "foreground", rgbHex(link)+"FF")

		s.SetState(gtk.StateFlagVisited)
		m.SetTagAttr("a:visited", "foreground", rgbHex(s.Color()))

		tags = m

		// Trick to cache this function shortly.
		cachedLinkTags = m
		glib.IdleAddPriority(glib.PriorityLow, func() { cachedLinkTags = nil })
	})

	return tags
}

func rgbHex(rgba *gdk.RGBA) string {
	return RGBHex(color.RGBA{
		R: uint8(0xFF * rgba.Red()),
		G: uint8(0xFF * rgba.Green()),
		B: uint8(0xFF * rgba.Blue()),
	})
}

// TextTagsMap describes a map of tag names to its attributes. It is used to
// declaratively construct a TextTagTable using NewTextTags.
type TextTagsMap map[string]TextTag

func isInternalKey(k string) bool { return strings.HasPrefix(k, "__internal") }

// SetTagAttr sets the attribute/property of the tag with the given name.
func (m TextTagsMap) SetTagAttr(name, attr string, value interface{}) {
	tag, ok := m[name]
	if !ok {
		tag = make(TextTag, 3)
		m[name] = tag
	}

	tag[attr] = value
}

// Combine adds all tags from other into m. If m already contains a tag that
// appears in other, then the tag is not overridden.
func (m TextTagsMap) Combine(other TextTagsMap) {
	for k, v := range other {
		// Ignore internals.
		if isInternalKey(k) {
			continue
		}

		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
}

// FromBuffer call FromTable on the buffer's tag table.
func (m TextTagsMap) FromBuffer(buffer *gtk.TextBuffer, name string) *gtk.TextTag {
	return m.FromTable(buffer.TagTable(), name)
}

// FromTable gets the tag with the given name from the given tag table, or if
// the tag doesn't exist, then a new one is added instead. If the name isn't
// known in either the table or the map, then the function will panic.
func (m TextTagsMap) FromTable(table *gtk.TextTagTable, name string) *gtk.TextTag {
	// Don't allow internal tags.
	if isInternalKey(name) {
		return nil
	}

	tag := table.Lookup(name)
	if tag != nil {
		return tag
	}

	tt, ok := m[name]
	if !ok {
		log.Panicln("unknown tag name", name)
		return nil
	}

	tag = tt.Tag(name)
	table.Add(tag)

	return tag
}

// TextTag describes a map of attribute/property name to its value for a
// TextTag. Attributes that need a -set suffix will be set to true
// automatically.
type TextTag map[string]interface{}

// FromTable gets the tag with the given name from the given table, else it'll
// make a new tag with the attributes from TextTag.
func (t TextTag) FromTable(table *gtk.TextTagTable, name string) *gtk.TextTag {
	tag := table.Lookup(name)
	if tag != nil {
		return tag
	}

	tag = t.Tag(name)
	table.Add(tag)

	return tag
}

// Tag creates a new text tag from the attributes.
func (t TextTag) Tag(name string) *gtk.TextTag {
	if isInternalKey(name) {
		log.Println("caller wants internal tag", name)
		return nil
	}

	if name == "" {
		name = t.Hash()
	}

	tag := gtk.NewTextTag(name)

	for k, v := range t {
		if isInternalKey(k) {
			continue
		}

		// Edge case.
		if v, ok := v.(string); ok && v == "" {
			continue
		}

		tag.SetObjectProperty(k, v)
	}

	return tag
}

// hack to guarantee thread safety while hashing. This is fine in most cases,
// because GTK is single-threaded. It is also fine when hashing is reasonably
// fast, and the initial slowdown time is barely noticeable in the first place.
var hashMutex sync.RWMutex

// Hash returns a 24-byte string of the text tag hashed.
func (t TextTag) Hash() string {
	const key = "__internal_hashcache"

	hashMutex.RLock()
	h, ok := t[key]
	hashMutex.RUnlock()

	if ok {
		return h.(string)
	}

	hashMutex.Lock()
	defer hashMutex.Unlock()

	// Double-check after acquisition.
	if h, ok := t[key]; ok {
		return h.(string)
	}

	hash := t.hashOnce()
	t[key] = hash
	return hash
}

func (t TextTag) hashOnce() string {
	hash := fnv.New128a()

	for k, v := range t {
		if isInternalKey(k) {
			continue
		}

		hash.Write([]byte(k))
		hash.Write([]byte(":"))
		fmt.Fprintln(hash, v)
	}

	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

// HashTag creates a tag inside the text tag table using the hash of the text
// tag attributes as the name. If the same tag has already been created, then it
// is returned.
func HashTag(table *gtk.TextTagTable, attrs TextTag) *gtk.TextTag {
	hash := "custom-" + attrs.Hash()

	if t := table.Lookup(hash); t != nil {
		return t
	}

	tag := attrs.Tag(hash)

	if !table.Add(tag) {
		log.Panicf("text tag hash collision %q", hash)
	}

	return tag
}

// darkThreshold is DarkColorHasher's value.
const darkThreshold = 0.65

// ColorIsDark determines if the given RGB colors are dark or not. It takes in
// colors of range [0.0, 1.0].
func ColorIsDark(r, g, b float64) bool {
	// Determine the value in the HSV colorspace. Code taken from
	// lucasb-eyer/go-colorful.
	v := math.Max(math.Max(r, g), b)
	return v <= darkThreshold
}

var darkStyles *gtk.StyleContext

// IsDarkTheme returns true if we're inside an application with a dark theme. A
// dark theme implies the background color is dark.
func IsDarkTheme() bool {
	var darkBg bool // default light theme

	bgcolor, ok := LookupColor(ThemeBackgroundColor)
	if ok {
		darkBg = ColorIsDark(
			float64(bgcolor.Red()),
			float64(bgcolor.Green()),
			float64(bgcolor.Blue()),
		)
	}

	return darkBg
}

// Constants for LookupColor.
const (
	ThemeBackgroundColor = "theme_bg_color"
	ThemeForegroundColor = "theme_fg_color"
)

// LookupColor looks up the color from a global StyleContext.
func LookupColor(color string) (rgba *gdk.RGBA, ok bool) {
	gtkutil.InvokeMain(func() {
		if darkStyles == nil {
			w := gtk.NewLabel("")
			darkStyles = w.StyleContext()
		}

		rgba, ok = darkStyles.LookupColor(color)
	})
	return
}
