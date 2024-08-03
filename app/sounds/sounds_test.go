package sounds

import (
	"context"
	"log/slog"
	"testing"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/neilotoole/slogt"
)

func TestPlay(t *testing.T) {
	injectLogger(t)

	enableCanberra = false
	t.Cleanup(func() { enableCanberra = true })

	app := app.New(context.Background(), "com.example.app", "app")
	app.ConnectActivate(func() {
		app.Hold()

		spamSounds := func() {
			// Plays are automatically debounced.
			Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
			// Play(app, Message)
		}

		var i int
		glib.TimeoutSecondsAdd(1, func() bool {
			spamSounds()
			i++

			if i >= 10 {
				app.Release()
				return false
			}

			return true
		})
	})

	app.Run(nil)
	// Output:
}

func injectLogger(t *testing.T) {
	t.Helper()

	oldLogger := slog.Default()
	newLogger := slogt.New(t)

	slog.SetDefault(newLogger)
	t.Cleanup(func() { slog.SetDefault(oldLogger) })
}
