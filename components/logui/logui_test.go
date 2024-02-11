package logui_test

import (
	"context"
	"log/slog"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/components/logui"
)

func Example() {
	app2 := app.New(context.Background(), "com.github.diamondburned.gotkit.components.logui", "logui")
	app2.ConnectActivate(func() {
		adw.Init()
		logui.Hook()

		slog.Debug("example debug message", "key", "value")
		slog.Info("example info message", "key", "value")
		slog.Warn("example warn message", "key", "value")
		slog.Error("example error message", "key", "value")

		win := app2.NewWindow()
		win.SetTitle("Log Viewer")
		win.SetChild(gtk.NewLabel("Log viewer demo"))
		win.Show()

		ctx := app.WithWindow(app2.Context(), win)
		logui.ShowDefaultViewer(ctx)
	})
	app2.Run(nil)

	// Output:
}
