// Package prefs provides a publish-subscription API for global settinsg
// management. It exists as a schema-less version of the GSettings API.
package prefs

import (
	"context"
	"encoding/json"
	"fmt"
	"go/doc"
	"log"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/utils/config"
	"github.com/pkg/errors"
	"golang.org/x/text/message"
)

// propRegistry is the global registry map.
var propRegistry = map[ID]Prop{}

// RegisterProp registers a property globally. This function should ideally be
// called only during init.
func RegisterProp(p Prop) {
	id := p.Meta().ID()

	if _, ok := propRegistry[id]; ok {
		log.Panicf("ID collision for property %q", id)
	}

	propRegistry[id] = p
}

// propOrder maps English prop names to the order integer.
type propOrder map[string]string

// sectionOrders maps a section name to all its prop orders.
var sectionOrders = map[string]propOrder{}

func OrderBefore(prop, isBefore Prop) {
	p := prop.Meta()
	b := isBefore.Meta()

	if p.EnglishSectionName() != b.EnglishSectionName() {
		panic("BUG: prop/before mismatch section name")
	}

	orders, ok := sectionOrders[p.EnglishSectionName()]
	if !ok {
		orders = propOrder{}
		sectionOrders[p.EnglishSectionName()] = orders
	}

	orders[p.EnglishName()] = b.EnglishName()
}

// Order registers the given names for ordering properties. It is only valid
// within the same sections.
func Order(props ...Prop) {
	for i, prop := range props[1:] {
		// slice where 1st item is popped off, so 1st is 2nd.
		OrderBefore(props[i], prop)
	}
}

var hidden = make(map[Prop]bool)

// Hide hides the given props from showing up in methods that snapshot
// properties. This is useful for the main application to hide the settings of
// certain libraries.
//
// Props that are hidden cannot be unhidden, so only call this method during
// initialization on a constant list of properties.
func Hide(props ...Prop) {
	for _, prop := range props {
		hidden[prop] = true
	}
}

// TODO: scrap this routine; just sort normally and scramble the slice
// afterwards.
func sectionPropOrder(orders propOrder, i, j string) bool {
	if orders != nil {
		// I honestly have no idea how I thought of this code. But I did.
		// https://go.dev/play/p/ONG4HbU_Rhl
		ibefore, iok := orders[i]
		jbefore, jok := orders[j]

		if iok && ibefore == j {
			return true
		}
		if jok && jbefore == i {
			return false
		}

		if iok {
			return sectionPropOrder(orders, ibefore, j)
		}
		if jok {
			return sectionPropOrder(orders, i, jbefore)
		}
	}

	return i < j
}

// LoadData loads the given JSON data (usually returned from ReadSavedData)
// directly into the global preference values.
func LoadData(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var props map[string]json.RawMessage
	if err := json.Unmarshal(data, &props); err != nil {
		return err
	}
	for k, blob := range props {
		prop, ok := propRegistry[ID(k)]
		if !ok {
			continue
		}
		if err := prop.UnmarshalJSON(blob); err != nil {
			return fmt.Errorf("error at %s: %w", k, err)
		}
	}
	return nil
}

// Snapshot describes a snapshot of the preferences state.
type Snapshot map[string]json.RawMessage

// TakeSnapshot takes a snapshot of the global preferences into a flat map. This
// function should only be called on the main thread, but the returned snapshot
// can be used anywhere.
func TakeSnapshot() Snapshot {
	v := make(map[string]json.RawMessage, len(propRegistry))
	for id, prop := range propRegistry {
		b, err := prop.MarshalJSON()
		if err != nil {
			log.Panicf("cannot marshal property %q: %s", id, err)
		}
		v[string(id)] = json.RawMessage(b)
	}
	return v
}

// JSON marshals the snapshot as JSON. Any error that arises from marshaling the
// JSON is assumed to be the user tampering with it.
func (s Snapshot) JSON() []byte {
	b, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		log.Panicln("prefs: cannot marshal snapshot:", err)
	}
	return b
}

func prefsPath(ctx context.Context) string {
	return app.FromContext(ctx).ConfigPath("prefs.json")
}

// Save atomically saves the snapshot to file.
func (s Snapshot) Save(ctx context.Context) error {
	return config.WriteFile(prefsPath(ctx), s.JSON())
}

// AsyncLoadSaved asynchronously loads the saved preferences.
func AsyncLoadSaved(ctx context.Context, done func(error)) {
	onDone := func(err error) {
		if done != nil {
			done(err)
		} else if err != nil {
			app.Error(ctx, err)
		}
	}

	gtkutil.Async(ctx, func() func() {
		data, err := ReadSavedData(ctx)
		if err != nil {
			return func() { onDone(errors.Wrap(err, "cannot read saved preferences")) }
		}

		return func() {
			err := LoadData(data)
			if err != nil {
				err = errors.Wrap(err, "cannot load saved preferences")
			}
			onDone(err)
		}
	})
}

// ReadSavedData reads the saved preferences from a predetermined location.
// Users should give the returned byte slice to LoadData. A nil byte slice is a
// valid value.
func ReadSavedData(ctx context.Context) ([]byte, error) {
	b, err := os.ReadFile(prefsPath(ctx))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		log.Println("cannot open prefs.json:", err)
		return nil, err
	}
	return b, nil
}

// ListedSection holds a list of properties returned from ListProperties.
type ListedSection struct {
	Name  string // localized
	Props []LocalizedProp

	name string // unlocalized
}

// LocalizedProp wraps Prop and localizes its name and description.
type LocalizedProp struct {
	Prop
	Name        string
	Description string
}

// ListProperties enumerates all known global properties into a map of
func ListProperties(ctx context.Context) []ListedSection {
	m := map[message.Reference][]Prop{}

	for _, prop := range propRegistry {
		if hidden[prop] {
			continue
		}

		meta := prop.Meta()
		m[meta.Section] = append(m[meta.Section], prop)
	}

	localize := func(ref message.Reference) string {
		if ref == nil {
			return ""
		}

		str := locale.S(ctx, ref)
		return docToText(str)
	}

	sections := make([]ListedSection, 0, len(m))

	for s, props := range m {
		section := ListedSection{
			Name:  localize(s),
			Props: make([]LocalizedProp, len(props)),
			name:  props[0].Meta().EnglishSectionName(),
		}

		for i, prop := range props {
			if hidden[prop] {
				continue
			}

			meta := prop.Meta()

			section.Props[i] = LocalizedProp{
				Prop:        prop,
				Name:        localize(meta.Name),
				Description: localize(meta.Description),
			}
		}

		orders := sectionOrders[section.name]

		sort.Slice(section.Props, func(i, j int) bool {
			iname := section.Props[i].Meta().EnglishName()
			jname := section.Props[j].Meta().EnglishName()
			return sectionPropOrder(orders, iname, jname)
		})

		sections = append(sections, section)
	}

	sort.Slice(sections, func(i, j int) bool {
		return sections[i].name < sections[j].name
	})

	return sections
}

func docToText(str string) string {
	str = unindent(str)

	var b strings.Builder
	doc.ToText(&b, str, "", "\t", math.MaxInt)

	str = b.String()
	str = strings.Trim(str, "\n")

	return str
}

func unindent(str string) string {
	lines := strings.Split(str, "\n")

	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := countByteUntil(line, '\t')
		if indent < minIndent || minIndent == -1 {
			minIndent = indent
		}
	}

	for i := range lines {
		for j := 0; j < minIndent; j++ {
			lines[i] = strings.TrimPrefix(lines[i], "\t")
		}
	}

	return strings.Join(lines, "\n")
}

func countByteUntil(str string, char byte) int {
	var i int
	for i < len(str) && str[i] == char {
		i++
	}
	return i
}
