package prefs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/locale"
	"golang.org/x/text/message"
)

// ErrInvalidAnyType is returned by a preference property if it has the wrong
// type.
var ErrInvalidAnyType = errors.New("incorrect value type")

// Bool is a preference property of type boolean.
type Bool struct {
	Pubsub
	PropMeta
	v uint32
}

// NewBool creates a new boolean with the given default value and properties.
func NewBool(v bool, prop PropMeta) *Bool {
	validateMeta(prop)

	b := &Bool{
		Pubsub:   *NewPubsub(),
		PropMeta: prop,

		v: boolToUint32(v),
	}

	RegisterProp(b)
	return b
}

// Publish publishes the new boolean.
func (b *Bool) Publish(v bool) {
	atomic.StoreUint32(&b.v, boolToUint32(v))
	b.Pubsub.Publish()
}

// Value loads the internal boolean.
func (b *Bool) Value() bool {
	return atomic.LoadUint32(&b.v) != 0
}

func (b *Bool) MarshalJSON() ([]byte, error) { return json.Marshal(b.Value()) }

func (b *Bool) UnmarshalJSON(blob []byte) error {
	var v bool
	if err := json.Unmarshal(blob, &v); err != nil {
		return err
	}
	b.Publish(v)
	return nil
}

// AnyValue implements Prop.
func (b *Bool) AnyValue() interface{} { return b.Value() }

// AnyPublish implements Prop.
func (b *Bool) AnyPublish(v interface{}) error {
	bv, ok := v.(bool)
	if !ok {
		return ErrInvalidAnyType
	}
	b.Publish(bv)
	return nil
}

func boolToUint32(b bool) (u uint32) {
	if b {
		u = 1
	}
	return
}

// CreateWidget creates a *gtk.Switch.
func (b *Bool) CreateWidget(ctx context.Context, save func()) gtk.Widgetter {
	sw := gtk.NewSwitch()
	sw.AddCSSClass("prefui-prop")
	sw.AddCSSClass("prefui-prop-bool")
	bindPropWidget(b, sw, "notify::active", propFuncs{
		save:    save,
		set:     func() { sw.SetActive(b.Value()) },
		publish: func() { b.Publish(sw.Active()) },
	})
	return sw
}

// WidgetIsLarge returns false.
func (b *Bool) WidgetIsLarge() bool { return false }

// Int is a preference property of type int.
type Int struct {
	Pubsub
	IntMeta
	v int32
}

// IntMeta wraps PropMeta for Int.
type IntMeta struct {
	Name        message.Reference
	Section     message.Reference
	Description message.Reference
	Min         int
	Max         int
	Slider      bool
}

// Meta returns the PropMeta for IntMeta. It implements Prop.
func (m IntMeta) Meta() PropMeta {
	return PropMeta{
		Name:        m.Name,
		Section:     m.Section,
		Description: m.Description,
	}
}

// NewInt creates a new int(32) with the given default value and properties.
func NewInt(v int, meta IntMeta) *Int {
	validateMeta(meta.Meta())

	b := &Int{
		Pubsub:  *NewPubsub(),
		IntMeta: meta,

		v: int32(v),
	}

	RegisterProp(b)
	return b
}

// Publish publishes the new int.
func (i *Int) Publish(v int) {
	atomic.StoreInt32(&i.v, int32(v))
	i.Pubsub.Publish()
}

// Value loads the internal int.
func (i *Int) Value() int {
	return int(atomic.LoadInt32(&i.v))
}

func (i *Int) MarshalJSON() ([]byte, error) { return json.Marshal(i.Value()) }

func (i *Int) UnmarshalJSON(b []byte) error {
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	i.Publish(v)
	return nil
}

// CreateWidget creates either a *gtk.Scale or a *gtk.SpinButton.
func (i *Int) CreateWidget(ctx context.Context, save func()) gtk.Widgetter {
	min := float64(i.Min)
	max := float64(i.Max)
	if i.Slider {
		slider := gtk.NewScaleWithRange(gtk.OrientationHorizontal, min, max, 1)
		slider.AddCSSClass("prefui-prop")
		slider.AddCSSClass("prefui-prop-int")
		bindPropWidget(i, slider, "changed", propFuncs{
			save:    save,
			set:     func() { slider.SetValue(float64(i.Value())) },
			publish: func() { i.Publish(int(math.Round(slider.Value()))) },
		})
		return slider
	} else {
		spin := gtk.NewSpinButtonWithRange(min, max, 1)
		spin.AddCSSClass("prefui-prop")
		spin.AddCSSClass("prefui-prop-int")
		bindPropWidget(i, spin, "value-changed", propFuncs{
			save:    save,
			set:     func() { spin.SetValue(float64(i.Value())) },
			publish: func() { i.Publish(spin.ValueAsInt()) },
		})
		return spin
	}
}

// WidgetIsLarge is true if Slider is true.
func (i *Int) WidgetIsLarge() bool { return i.Slider }

// StringMeta is the metadata of a string.
type StringMeta struct {
	Name        message.Reference
	Section     message.Reference
	Description message.Reference
	Placeholder message.Reference
	Validate    func(string) error
	Multiline   bool
}

// Meta returns the PropMeta for StringMeta. It implements Prop.
func (m StringMeta) Meta() PropMeta {
	return PropMeta{
		Name:        m.Name,
		Section:     m.Section,
		Description: m.Description,
	}
}

// String is a preference property of type string.
type String struct {
	Pubsub
	StringMeta
	val string
	mut sync.Mutex
}

// NewString creates a new String instance.
func NewString(def string, prop StringMeta) *String {
	validateMeta(prop.Meta())

	l := &String{
		Pubsub:     *NewPubsub(),
		StringMeta: prop,

		val: def,
	}

	if prop.Validate != nil {
		if err := prop.Validate(def); err != nil {
			log.Panicf("default value %q fails validation: %v", def, err)
		}
	}

	RegisterProp(l)
	return l
}

// Publish publishes the new string value. An error is returned and nothing is
// published if the string fails the verifier.
func (s *String) Publish(v string) error {
	if s.Validate != nil {
		if err := s.Validate(v); err != nil {
			return err
		}
	}

	s.mut.Lock()
	s.val = v
	s.mut.Unlock()

	s.Pubsub.Publish()
	return nil
}

// Value returns the internal string value.
func (s *String) Value() string {
	s.mut.Lock()
	defer s.mut.Unlock()

	return s.val
}

func (s *String) MarshalJSON() ([]byte, error) { return json.Marshal(s.Value()) }

func (s *String) UnmarshalJSON(blob []byte) error {
	var v string
	if err := json.Unmarshal(blob, &v); err != nil {
		return err
	}
	s.Publish(v)
	return nil
}

// CreateWidget creates either a *gtk.Entry or a *gtk.TextView.
func (s *String) CreateWidget(ctx context.Context, save func()) gtk.Widgetter {
	// TODO: multiline
	entry := gtk.NewEntry()
	entry.AddCSSClass("prefui-prop")
	entry.AddCSSClass("prefui-prop-string")
	entry.SetWidthChars(10)
	entry.SetPlaceholderText(locale.S(ctx, s.Placeholder))
	entry.ConnectChanged(func() {
		setEntryIcon(entry, "object-select", "")
	})
	bindPropWidget(s, entry, "activate,icon-press", propFuncs{
		save: save,
		set: func() {
			entry.SetText(s.Value())
		},
		publish: func() bool {
			if err := s.Publish(entry.Text()); err != nil {
				setEntryIcon(entry, "dialog-error", "Error: "+err.Error())
				return false
			} else {
				setEntryIcon(entry, "object-select", "")
				return true
			}
		},
	})
	return entry
}

// WidgetIsLarge returns true.
func (s *String) WidgetIsLarge() bool { return true }

func setEntryIcon(entry *gtk.Entry, icon, text string) {
	entry.SetIconFromIconName(gtk.EntryIconSecondary, icon)
	entry.SetIconTooltipText(gtk.EntryIconSecondary, text)
}

// EnumList is a preference property of type stringer.
type EnumList struct {
	Pubsub
	EnumListMeta
	val string
	mut sync.RWMutex
}

// EnumListMeta is the metadata of an EnumList.
type EnumListMeta struct {
	PropMeta
	Validate func(string) error
	Options  []string
}

// NewEnumList creates a new EnumList instance.
func NewEnumList(def string, prop EnumListMeta) *EnumList {
	l := &EnumList{
		Pubsub:       *NewPubsub(),
		EnumListMeta: prop,

		val: def,
	}

	if !l.IsValid(def) {
		log.Panicf("invalid default value %q, possible: %q.", def, l.Options)
	}

	RegisterProp(l)
	return l
}

// Publish publishes the new value. If the value isn't within Options, then the
// method will panic.
func (l *EnumList) Publish(v string) {
	if !l.IsValid(v) {
		log.Panicf("publishing invalid value %q, possible: %q.", v, l.Options)
	}

	l.mut.Lock()
	l.val = v
	l.mut.Unlock()

	l.Pubsub.Publish()
}

// Value gets the current enum value.
func (l *EnumList) Value() string {
	l.mut.RLock()
	defer l.mut.RUnlock()

	return l.val
}

func (l *EnumList) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.Value())
}

func (l *EnumList) UnmarshalJSON(blob []byte) error {
	var str string

	if err := json.Unmarshal(blob, &str); err != nil {
		return fmt.Errorf("cannot unmarshal enum %q: %v", blob, err)
	}

	if !l.IsValid(str) {
		return fmt.Errorf("enum %q is not a known values", str)
	}

	l.Publish(str)
	return nil
}

// IsValid returns true if the given value is a valid enum value.
func (l *EnumList) IsValid(str string) bool {
	for _, opt := range l.Options {
		if opt == str {
			return true
		}
	}
	return false
}

// CreateWidget creates either a *gtk.Entry or a *gtk.TextView.
func (l *EnumList) CreateWidget(ctx context.Context, save func()) gtk.Widgetter {
	combo := gtk.NewComboBoxText()
	combo.AddCSSClass("prefui-prop")
	combo.AddCSSClass("prefui-prop-enumlist")
	for _, text := range l.Options {
		combo.Append(text, text)
	}
	bindPropWidget(l, combo, "changed", propFuncs{
		save: save,
		set: func() {
			combo.SetActiveID(l.Value())
		},
		publish: func() bool {
			l.Publish(combo.ActiveID())
			return true
		},
	})
	return combo
}

// WidgetIsLarge returns false.
func (l *EnumList) WidgetIsLarge() bool { return false }

type propFuncs struct {
	save    func()
	set     func()
	publish interface{}
}

func bindPropWidget(p Prop, w gtk.Widgetter, changed string, funcs propFuncs) {
	var paused bool

	activate := func() {
		if paused {
			return
		}

		switch publish := funcs.publish.(type) {
		case func():
			publish()
		case func() bool:
			if !publish() {
				return
			}
		case func() error:
			if err := publish(); err != nil {
				return
			}
		default:
			log.Panicf("unknown publish callback type %T", publish)
		}

		funcs.save()
	}

	for _, signal := range strings.Split(changed, ",") {
		w.Connect(signal, activate)
	}

	p.Pubsubber().SubscribeWidget(w, func() {
		paused = true
		funcs.set()
		paused = false
	})
}
