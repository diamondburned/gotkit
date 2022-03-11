package prefs

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"unicode"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Prop describes a property type.
type Prop interface {
	json.Marshaler
	json.Unmarshaler
	// Meta returns the property's meta.
	Meta() PropMeta
	// Pubsubber returns the internal publisher/subscriber instance.
	Pubsubber() *Pubsub
	// CreateWidget creates the widget containing the value of this property.
	CreateWidget(ctx context.Context, save func()) gtk.Widgetter
	// WidgetIsLarge returns true if the widget created by CreateWidget should
	// take up space and shouldn't be inlined.
	WidgetIsLarge() bool
}

// PropMeta describes the metadata of a preference value.
type PropMeta struct {
	Name        message.Reference
	Section     message.Reference
	Description message.Reference
	// Hidden, if true, will hide the option by default. This is useful for
	// libraries to add preferences in without hindering the user.
	Hidden bool
}

// Meta returns itself. It implements Prop.
func (p PropMeta) Meta() PropMeta { return p }

var nullPrinter = message.NewPrinter(
	language.Und,
	message.Catalog(message.DefaultCatalog),
)

func nolocalize(ref message.Reference) string {
	if str, ok := ref.(string); ok {
		return str
	}
	return nullPrinter.Sprint(ref)
}

// PropID implements Prop.
func (p PropMeta) ID() ID {
	id := ID(Slugify(nolocalize(p.Section)))
	id += "/"
	id += ID(Slugify(nolocalize(p.Name)))
	return id
}

// EnglishName returns the unlocalized name.
func (p PropMeta) EnglishName() string {
	return nolocalize(p.Name)
}

// EnglishSectionName returns the unlocalized section name.
func (p PropMeta) EnglishSectionName() string {
	return nolocalize(p.Section)
}

func validateMeta(p PropMeta) {
	if p.Name == nil || p.Name == "" {
		log.Panicln("missing prop name")
	}
	if p.Section == nil || p.Section == "" {
		log.Panicln("missing prop section")
	}
}

// ID describes a property ID type.
type ID Slug

// Slug describes a particular slug format type.
type Slug string

// Slugify turns any string into an ID string.
func Slugify(any string) Slug {
	return Slug(strings.Map(slugify, any))
}

func slugify(r rune) rune {
	if r == '/' {
		return '-'
	}
	if unicode.IsSpace(r) {
		return '-'
	}
	return unicode.ToLower(r)
}
