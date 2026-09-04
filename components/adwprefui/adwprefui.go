package adwprefui

import (
	"context"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/utils/config"
	"github.com/pkg/errors"
)

// Dialog is a widget that lists all known preferences in a dialog.
type Dialog struct {
	*adw.PreferencesDialog

	ctx      context.Context
	sections []*adw.PreferencesGroup
	saver    config.ConfigStore
}

var currentDialog *Dialog

// ShowDialog shows the preferences dialog.
func ShowDialog(ctx context.Context) {
	// LibAdwaita handles everything related to Display context (onActivate, onDestroy,...)
	// So we just store the context for custom Widget creation (L112)
	if currentDialog == nil {
		currentDialog = newDialog(ctx)
	} else {
		currentDialog.ctx = ctx
	}

	currentDialog.Present(app.GTKWindowFromContext(ctx))
}

func configSnapshotter(ctx context.Context) func() (save func()) {
	return func() func() {
		snapshot := prefs.TakeSnapshot()
		return func() {
			if err := snapshot.Save(ctx); err != nil {
				app.Error(ctx, errors.Wrap(err, "cannot save prefs"))
			}
		}
	}
}

// newDialog creates a new preferences UI.
func newDialog(ctx context.Context) *Dialog {
	d := Dialog{ctx: ctx}

	d.saver = config.NewConfigStore(configSnapshotter(ctx))

	preferencesPage := adw.NewPreferencesPage()  // TODO maybe later:
												 // Provide a way for preferences to choose on which page they want to appear
	preferencesPage.SetTitle(locale.Get("Preferences"))
	preferencesPage.SetIconName("settings-symbolic")

	sections := prefs.ListProperties(ctx)
	d.sections = make([]*adw.PreferencesGroup, len(sections))  // Sections are PreferencesGroup
	for i, section := range sections {
		d.sections[i] = newSection(&d, section)
		preferencesPage.Add(d.sections[i])
	}

	d.PreferencesDialog = adw.NewPreferencesDialog()
	d.PreferencesDialog.SetTitle(locale.Get("Preferences"))
	d.PreferencesDialog.SetSearchEnabled(true)
	d.PreferencesDialog.Add(preferencesPage)

	if app.IsDevel() {
		d.Dialog.AddCSSClass("devel")
	}

	return &d
}

func (d *Dialog) save() {
	d.saver.Save()
}

type dialogSaver Dialog

// Because PreferencesDialog is less customizable, the spinner is now lost
// Let's just use Dialog title to inform about saving status
func (d *dialogSaver) SaveBegin() {
	d.PreferencesDialog.SetTitle(locale.Get("Saving preferences..."))
}

func (d *dialogSaver) SaveEnd() {
	d.PreferencesDialog.SetTitle(locale.Get("Preferences"))
}

func newSection(d *Dialog, sect prefs.ListedSection) *adw.PreferencesGroup {
	s := adw.NewPreferencesGroup()

	for _, prop := range sect.Props {
		s.Add(newPropRow(d, prop))
	}
	s.SetTitle(sect.Name)
	// We now have the ability to provide a section Description, but unused for now
	//s.SetDescription(sect.Description)
	return s
}

func newPropRow(d *Dialog, prop prefs.LocalizedProp) *adw.PreferencesRow {
	row := adw.PreferencesRow{}
	metadataWidget := adw.NewActionRow()

	action := prop.CreateWidget(d.ctx, d.save)
	gtk.BaseWidget(action).SetVAlign(gtk.AlignCenter)

	// Layout differ if a full-length widget is requested:
	//  - If inline, the ActionRow has a suffix
	//  - If large, the widget and ActionRow are placed inside a VBox, inside a PreferencesRow
	if prop.WidgetIsLarge() {
		vbox := gtk.NewBox(gtk.OrientationVertical, 0)
		vbox.Append(metadataWidget)
		vbox.Append(action)
		row = *adw.NewPreferencesRow()
		row.SetChild(vbox)
	} else {
		row = metadataWidget.PreferencesRow
		metadataWidget.AddSuffix(action)
	}

	metadataWidget.SetTitle(prop.Name)
	metadataWidget.SetUseMarkup(true)

	if prop.Description != "" {
		metadataWidget.SetSubtitle(prop.Description)
	}

	return &row
}
