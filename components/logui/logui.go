package logui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/components/autoscroll"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

// Viewer is a TextView dialog that views a particular log buffer in real time.
type Viewer struct {
	*adw.ApplicationWindow
	View  *gtk.ColumnView
	Model *LogListModel

	ctx context.Context
}

// ShowDefaultViewer calls NewDefaultViewer then Show.
func ShowDefaultViewer(ctx context.Context) {
	viewer := NewDefaultViewer(ctx)
	viewer.SetHideOnClose(false)
	viewer.SetDestroyWithParent(true)
	viewer.Show()
}

// NewDefaultViewer creates a new viewer on the default buffer.
func NewDefaultViewer(ctx context.Context) *Viewer {
	return NewViewer(ctx, DefaultLogHandler().ListModel())
}

var _ = cssutil.WriteCSS(`
	/*
	.logui-time,
	.logui-level {
		font-family: monospace;
	}
	*/

	.logui-column-view {
		font-family: monospace;
	}
	.logui-column-view cell {
		padding: 2px;
	}
	.logui-column-view cell:first-child {
		padding: 2px 6px;
	}

	.logui-dark .logui-level-debug { color: #9fa8da; }
	.logui-dark .logui-level-info  { color: #a5d6a7; }
	.logui-dark .logui-level-warn  { color: #ffcc80; }
	.logui-dark .logui-level-error { color: #ef9a9a; }

	.logui-light .logui-level-debug { color: #1a237e; }
	.logui-light .logui-level-info  { color: #004d40; }
	.logui-light .logui-level-warn  { color: #e65100; }
	.logui-light .logui-level-error { color: #b71c1c; }
`)

// NewViewer creates a new log viewer dialog.
func NewViewer(ctx context.Context, model *LogListModel) *Viewer {
	v := Viewer{Model: model, ctx: ctx}

	view := gtk.NewColumnView(gtk.NewMultiSelection(model.ListModel))
	view.AddCSSClass("logui-column-view")
	view.SetShowRowSeparators(false)
	view.SetShowColumnSeparators(false)
	view.SetEnableRubberband(true)
	view.SetHExpand(true)
	view.SetVExpand(true)
	view.SetSizeRequest(500, -1)
	view.SetObjectProperty("header-factory", (*coreglib.Object)(nil))
	view.AppendColumn(gtk.NewColumnViewColumn("Time", newTimeColumnFactory()))
	view.AppendColumn(gtk.NewColumnViewColumn("Level", newLevelColumnFactory()))
	msgColumn := gtk.NewColumnViewColumn("Message", newMessageColumnFactory())
	msgColumn.SetExpand(true)
	view.AppendColumn(msgColumn)
	view.AppendColumn(gtk.NewColumnViewColumn("Attrs", newAttrsColumnFactory()))

	v.View = view

	scroll := autoscroll.NewWindow()
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetPropagateNaturalWidth(true)
	scroll.SetPropagateNaturalHeight(true)
	scroll.SetChild(view)
	scroll.ScrollToBottom()

	copyButton := gtk.NewButtonFromIconName("edit-copy-symbolic")
	copyButton.SetTooltipText(locale.Get("Copy logs"))
	copyButton.SetActionName("win.copy")

	saveButton := gtk.NewButtonFromIconName("document-save-as-symbolic")
	saveButton.SetTooltipText(locale.Get("Save logs as..."))
	saveButton.SetActionName("win.save")

	header := adw.NewHeaderBar()
	header.PackStart(copyButton)
	header.PackStart(saveButton)

	toolbar := adw.NewToolbarView()
	toolbar.AddTopBar(header)
	toolbar.SetContent(scroll)

	win := app.GTKWindowFromContext(ctx)
	app := app.FromContext(ctx)

	v.ApplicationWindow = adw.NewApplicationWindow(app.Application)
	v.ApplicationWindow.AddCSSClass("logui-viewer")
	v.ApplicationWindow.SetTransientFor(win)
	v.ApplicationWindow.SetModal(true)
	v.ApplicationWindow.SetHideOnClose(false)
	v.ApplicationWindow.SetDestroyWithParent(true)
	v.ApplicationWindow.SetTitle(locale.Get("Logs"))
	v.ApplicationWindow.SetDefaultSize(500, 400)
	v.ApplicationWindow.SetContent(toolbar)

	styles := adw.StyleManagerGetDefault()
	updateDark := func() {
		if styles.Dark() {
			v.ApplicationWindow.AddCSSClass("logui-dark")
			v.ApplicationWindow.RemoveCSSClass("logui-light")
		} else {
			v.ApplicationWindow.AddCSSClass("logui-light")
			v.ApplicationWindow.RemoveCSSClass("logui-dark")
		}
	}
	updateDark()

	darkSignal := styles.NotifyProperty("dark", updateDark)
	v.ApplicationWindow.ConnectDestroy(func() { styles.HandlerDisconnect(darkSignal) })

	gtkutil.AddActions(v, map[string]func(){
		"close": func() { v.Close() },
		"copy":  func() { v.copyAll() },
		"save":  func() { v.saveAs() },
	})
	gtkutil.AddActionShortcuts(v, map[string]string{
		"Escape":     "win.close",
		"<Control>c": "win.copy",
		"<Control>s": "win.save",
	})

	return &v
}

func (v *Viewer) copyAll() {
	// TODO: copy only the selected items

	content := RecordsToString(v.Model.AllItems())

	display := gdk.DisplayGetDefault()

	clipboard := display.Clipboard()
	clipboard.SetText(content)
}

func (v *Viewer) saveAs() {
	content := RecordsToString(v.Model.AllItems())

	filePicker := gtk.NewFileChooserNative(
		app.FromContext(v.ctx).SuffixedTitle(locale.Get("Save Logs")),
		&v.ApplicationWindow.Window,
		gtk.FileChooserActionSave,
		locale.Get("Save"),
		locale.Get("Cancel"))
	filePicker.SetCreateFolders(true)
	filePicker.SetCurrentName("logs.txt")
	filePicker.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseAccept) {
			return
		}

		folderPath := filePicker.CurrentFolder().Path()
		fileName := filePicker.CurrentName()
		filePath := filepath.Join(folderPath, fileName)

		go func() {
			if err := os.WriteFile(filePath, []byte(content), 0640); err != nil {
				app.Error(v.ctx, fmt.Errorf("failed to save logs: %w", err))
			}
		}()
	})
	filePicker.Show()
}

func newTimeColumnFactory() *gtk.ListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(item *gtk.ListItem) {
		label := gtk.NewLabel("")
		label.AddCSSClass("logui-time")
		label.SetYAlign(0)
		item.SetChild(label)
	})
	factory.ConnectBind(func(item *gtk.ListItem) {
		record := LogListModelType.ObjectValue(item.Item())
		label := item.Child().(*gtk.Label)
		label.SetText(record.Time.Format("15:04:05.000"))
	})
	factory.ConnectTeardown(func(item *gtk.ListItem) {
		item.SetChild(nil)
	})
	return &factory.ListItemFactory
}

func newLevelColumnFactory() *gtk.ListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(item *gtk.ListItem) {
		label := gtk.NewLabel("")
		label.AddCSSClass("logui-level")
		label.SetXAlign(0)
		label.SetYAlign(0)
		item.SetChild(label)
	})
	factory.ConnectBind(func(item *gtk.ListItem) {
		record := LogListModelType.ObjectValue(item.Item())
		level := record.Level.String()

		label := item.Child().(*gtk.Label)
		label.SetText(level)

		switch {
		case strings.HasPrefix(level, "DEBUG"):
			label.SetCSSClasses([]string{"logui-level", "logui-level-warn"})
		case strings.HasPrefix(level, "INFO"):
			label.SetCSSClasses([]string{"logui-level", "logui-level-info"})
		case strings.HasPrefix(level, "WARN"):
			label.SetCSSClasses([]string{"logui-level", "logui-level-warn"})
		case strings.HasPrefix(level, "ERROR"):
			label.SetCSSClasses([]string{"logui-level", "logui-level-error"})
		default:
			label.SetCSSClasses([]string{"logui-level"})
		}
	})
	factory.ConnectTeardown(func(item *gtk.ListItem) {
		item.SetChild(nil)
	})
	return &factory.ListItemFactory
}

func newMessageColumnFactory() *gtk.ListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(item *gtk.ListItem) {
		label := gtk.NewLabel("")
		label.AddCSSClass("logui-message")
		label.SetWrap(false)
		label.SetXAlign(0)
		label.SetYAlign(0)
		item.SetChild(label)
	})
	factory.ConnectBind(func(item *gtk.ListItem) {
		record := LogListModelType.ObjectValue(item.Item())
		label := item.Child().(*gtk.Label)
		label.SetText(record.Message)
	})
	factory.ConnectTeardown(func(item *gtk.ListItem) {
		item.SetChild(nil)
	})
	return &factory.ListItemFactory
}

func newAttrsColumnFactory() *gtk.ListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	factory.ConnectSetup(func(item *gtk.ListItem) {
		grid := gtk.NewGrid()
		grid.SetRowHomogeneous(true)
		grid.SetColumnHomogeneous(true)
		grid.SetRowSpacing(4)
		grid.SetColumnSpacing(4)

		label := gtk.NewLabel("")
		label.AddCSSClass("logui-attrs")
		label.SetWrap(false)
		label.SetYAlign(0)
		label.SetXAlign(0)

		item.SetChild(label)
	})
	factory.ConnectBind(func(item *gtk.ListItem) {
		record := LogListModelType.ObjectValue(item.Item())
		label := item.Child().(*gtk.Label)

		var text strings.Builder
		record.Attrs(func(attr slog.Attr) bool {
			text.WriteString(attr.Key + "=" + attr.Value.String() + "\n")
			return true
		})
		str := strings.TrimSpace(text.String())
		label.SetText(str)
	})
	factory.ConnectTeardown(func(item *gtk.ListItem) {
		item.SetChild(nil)
	})
	return &factory.ListItemFactory
}
