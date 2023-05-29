package prefs

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"unicode"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
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
	Name        locale.Localized
	Section     locale.Localized
	Description locale.Localized
}

// Meta returns itself. It implements Prop.
func (p PropMeta) Meta() PropMeta { return p }

// PropID implements Prop.
func (p PropMeta) ID() ID {
	id := ID(Slugify(string(p.Section)))
	id += "/"
	id += ID(Slugify(string(p.Name)))
	return id
}

// EnglishName returns the unlocalized name.
func (p PropMeta) EnglishName() string {
	return string(p.Name)
}

// EnglishSectionName returns the unlocalized section name.
func (p PropMeta) EnglishSectionName() string {
	return string(p.Section)
}

func validateMeta(p PropMeta) {
	if p.Name == "" {
		log.Panicln("missing prop name")
	}
	if p.Section == "" {
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
