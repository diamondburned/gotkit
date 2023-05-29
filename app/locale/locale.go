package locale

import (
	"fmt"
	"io/fs"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/po"
	"github.com/leonelquinteros/gotext"

	mergedfs "github.com/yalue/merged_fs"
)

var current = gotext.NewLocale("", "C")

// LoadLocale loads the locale from the given filesystem. It will try to find
// the best match for the current locale.
func LoadLocale(localeFSes ...fs.FS) {
	localeFSes = append(localeFSes, po.FS)
	localeFS := mergedfs.MergeMultiple(localeFSes...)

	// TODO: allow option to scan $XDG_DATA_DIRS/locale. For now, we'll embed
	// the locale files.
	locale := "en_US"

	// Try to find best match.
	for _, lang := range glib.GetLanguageNames() {
		// Sometimes, the locale is in the form of "en_US.UTF-8". We'll try to
		// cut it down to "en_US" and see if it exists.
		lang = gotext.SimplifiedLocale(lang)

		if _, err := fs.Stat(localeFS, lang); err == nil {
			locale = lang
			break
		}

		// Otherwise, continue. GTK will cut it down to "en" for us.
	}

	current = gotext.NewLocaleFS(locale, localeFS)
}

// LoadCustomLocale loads the locale from the given filesystem.
func LoadCustomLocale(locale string, localeFS fs.FS) {
	current = gotext.NewLocaleFS(locale, localeFS)
}

// Get returns the translated string from the given reference.
func Get(str string, vars ...any) string {
	return current.Get(str, vars...)
}

// GetFromDomain returns the translated string from the given reference and
// domain. Use this if your string is in a different domain.
func GetFromDomain(domain, str string, vars ...any) string {
	return current.GetD(domain, str, vars...)
}

// Sprintf is an alias for Get.
func Sprintf(str string, vars ...any) string {
	return current.Get(str, vars...)
}

// Current returns the current locale.
func Current() *gotext.Locale {
	return current
}

/* TODO: implement Plural
// Plural formats the string in plural form.
func Plural(ctx context.Context, one, many message.Reference, n int) string {
	// I don't know how x/text/plural works.
	p := FromContext(ctx)
	if n == 1 {
		return p.Sprintf(one, n)
	}
	return p.Sprintf(many, n)
}
*/

// Localized is a string that can be localized.
// Its String() method will return the localized string.
type Localized string

// String implements fmt.Stringer. It returns the localized string.
func (l Localized) String() string {
	return Get(string(l))
}

func (l Localized) GoString() string {
	return fmt.Sprintf("locale.Localized(%q)", string(l))
}
