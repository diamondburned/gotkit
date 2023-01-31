package sounds_test

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/sounds"
)

func ExamplePlay() {
	app := app.New(context.Background(), "com.example.app", "app")
	app.ConnectActivate(func() {
		// Plays are automatically debounced.
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)
		sounds.Play(app, sounds.Message)

		app.Hold()
		glib.TimeoutSecondsAdd(1, app.Release)
	})

	app.Run(nil)
	// Output:
}
