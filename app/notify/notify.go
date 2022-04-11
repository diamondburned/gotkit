// Package notify provides an API to send declarative notifications as well as
// playing sounds when they're sent.
package notify

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/app/sounds"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
)

// Icon is a type for a notification icon.
type Icon interface {
	async() bool
	icon() gio.Iconner // can return nil
}

type iconName string

// IconName is a notification icon that follows the system icon
// theme.
func IconName(name string) Icon { return iconName(name) }

func (n iconName) async() bool { return false }

func (n iconName) icon() gio.Iconner {
	if n == "" {
		return nil
	}
	return gio.NewThemedIcon(string(n))
}

// MaxIconSize is the maximum size of the notification icon to give to
// the gio.Icon.
const MaxIconSize = 64

type iconURL struct {
	fallbackIcon iconName
	loadingIcon  <-chan *gio.BytesIcon
	finishedIcon *gio.BytesIcon
}

// IconURL creates a notification icon that is an image fetched online. The
// image is fetched using imgutil.GETPixbuf.
func IconURL(ctx context.Context, url string, fallback iconName) Icon {
	if url == "" {
		return fallback
	}

	loadingIcon := make(chan *gio.BytesIcon, 1)
	go func() {
		defer close(loadingIcon)

		ctx := imgutil.WithOpts(ctx,
			imgutil.WithRescale(MaxIconSize, MaxIconSize),
		)

		p, err := imgutil.GETPixbuf(ctx, url)
		if err != nil {
			log.Println("cannot GET notification icon URL:", err)
			return
		}

		b, err := p.SaveToBufferv("png", []string{"compression"}, []string{"0"})
		if err != nil {
			log.Println("cannot save notification icon URL as PNG:", err)
			return
		}

		loadingIcon <- gio.NewBytesIcon(glib.NewBytesWithGo(b))
	}()

	return iconURL{
		fallbackIcon: fallback,
		loadingIcon:  loadingIcon,
	}
}

func (n iconURL) async() bool {
	return true
}

func (n iconURL) icon() gio.Iconner {
	if n.finishedIcon != nil {
		return n.finishedIcon
	}

	icon, ok := <-n.loadingIcon
	if ok {
		n.finishedIcon = icon
		return n.finishedIcon
	}

	return n.fallbackIcon.icon()
}

// Sound is a type for a notification sound.
type Sound string

// Known notification sound constants.
const (
	NoSound      Sound = ""
	BellSound    Sound = sounds.Bell
	MessageSound Sound = sounds.Message
)

// ID is a type for a notification ID. It exists so convenient
// hashing functions can exist. If the ID is empty, then GTK will internally
// generate a new one. There's no way to recall/change the notification then.
type ID string

// HashID are created from hashing the given inputs. This is useful
// for generating short notification IDs that are uniquely determined by the
// inputs.
func HashID(keys ...interface{}) ID {
	// We're not actually hashing any of this. We don't need to.
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprint(&b, key)
		b.WriteByte(';')
	}
	return ID(b.String())
}

// Action is an action of a notification.
type Action struct {
	ActionID string
	Argument *glib.Variant
}

// Notification is a data structure for a notification. A GNotification object
// is created from this type.
type Notification struct {
	ID    ID
	Title string // required
	Body  string
	// Icon is the notification icon. If it's nil, then the application's icon
	// is used.
	Icon Icon
	// Action is the action to activate if the notification is clicked.
	Action Action
	// Priority is the priority of the notification.
	Priority gio.NotificationPriority
	// Sound, if true, will ring a sound. If it's an empty string, then no sound
	// is played.
	Sound Sound
}

// async returns true if the notification must be constructed within a
// goroutine.
func (n *Notification) async() bool {
	return (n.Icon != nil && n.Icon.async())
}

func (n *Notification) asGio() *gio.Notification {
	if n.Title == "" {
		panic("notification missing Title")
	}

	notification := gio.NewNotification(n.Title)

	if n.Body != "" {
		notification.SetBody(n.Body)
	}

	if n.Priority != 0 {
		notification.SetPriority(n.Priority)
	}

	if n.Icon != nil {
		if icon := n.Icon.icon(); icon != nil {
			notification.SetIcon(icon)
		}
	}

	if n.Action != (Action{}) {
		notification.SetDefaultActionAndTarget(n.Action.ActionID, n.Action.Argument)
	}

	return notification
}

// ShowNotification is a preference.
var ShowNotification = prefs.NewBool(true, prefs.PropMeta{
	Name:    "Show Notifications",
	Section: "Application",
	Description: "Show a notification for messages that mention the user. " +
		"No notifications are triggered if the user is focused on the window",
})

// PlayNotificationSound is a preference.
var PlayNotificationSound = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Play Notification Sound",
	Section:     "Application",
	Description: "Play a sound every time a notification pops up.",
})

func init() {
	prefs.Order(ShowNotification, PlayNotificationSound)
}

func (n *Notification) playSound(app *app.Application) {
	if PlayNotificationSound.Value() && n.Sound != NoSound {
		sounds.Play(app, string(n.Sound))
	}
}

// Not making Send take in a context.Context is a fairly arbitrary decision that
// is probably a bad idea in hindsight.

// Send sends the notification to the application.
func (n *Notification) Send(app *app.Application) {
	if !ShowNotification.Value() {
		return
	}

	n.playSound(app)

	if n.async() {
		go app.SendNotification(string(n.ID), n.asGio())
	} else {
		app.SendNotification(string(n.ID), n.asGio())
	}
}

// Send is a convenient function.
func Send(ctx context.Context, n Notification) {
	n.Send(app.FromContext(ctx))
}
