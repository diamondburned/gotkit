package locale

import (
	"fmt"
	"io/fs"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/leonelquinteros/gotext"

	mergedfs "github.com/yalue/merged_fs"
)

var current = gotext.NewLocale("", "C")
var localeDomains = map[string]fs.FS{}

// RegisterLocaleDomain registers a locale domain. This is used to load
// translations from a different domain.
func RegisterLocaleDomain(domain string, fs fs.FS) {
	if domain == "default" {
		panic("domain `default' cannot be registered, use LoadLocale instead")
	}
	if _, ok := localeDomains[domain]; ok {
		panic("domain " + domain + " already registered")
	}
	localeDomains[domain] = fs
}

// LoadLocale loads the locale from the given filesystem. It will try to find
// the best match for the current locale.
func LoadLocale(defaultLocaleDomain fs.FS) {
	if _, ok := localeDomains["default"]; ok {
		panic("domain `default' already registered (LoadLocale called twice?)")
	}
	localeDomains["default"] = defaultLocaleDomain

	localeFSes := make([]fs.FS, 0, len(localeDomains))
	for _, fs := range localeDomains {
		localeFSes = append(localeFSes, fs)
	}

	localeFS := mergedfs.MergeMultiple(localeFSes...)

	// TODO: allow option to scan $XDG_DATA_DIRS/locale. For now, we'll embed
	// the locale files.
	var locale string

	// Try to find best match.
	for _, lang := range glib.GetLanguageNames() {
		if _, err := fs.Stat(localeFS, lang); err == nil {
			locale = lang
			break
		}

		// Otherwise, continue. GTK will cut it down to "en" for us.
	}

	if locale == "" {
		return
	}

	current = gotext.NewLocaleFS(locale, localeFS)
	for domain := range localeDomains {
		current.AddDomain(domain)
	}
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
