package prefui

import (
	"context"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/utils/config"
	"github.com/pkg/errors"
)

// Dialog is a widget that lists all known preferences in a dialog.
type Dialog struct {
	*gtk.Dialog
	ctx context.Context

	header   *gtk.HeaderBar
	box      *gtk.Box
	search   *gtk.SearchBar
	loading  *gtk.Spinner
	sections []*section

	saver config.ConfigStore
}

var currentDialog *Dialog

// ShowDialog shows the preferences dialog.
func ShowDialog(ctx context.Context) {
	if currentDialog != nil {
		currentDialog.ctx = ctx
		currentDialog.Present()
		return
	}

	dialog := newDialog(ctx)
	dialog.ConnectClose(func() {
		currentDialog = nil
		dialog.Destroy()
	})
	dialog.Show()
}

var _ = cssutil.WriteCSS(`
	.prefui-section-box:not(:first-child) {
		border-top: 1px solid @borders;
	}

	.prefui-heading {
		margin: 16px;
		margin-bottom: 10px;

		font-weight: bold;
		font-size: 0.95em;

		color: mix(@theme_fg_color, @theme_bg_color, 0.15);
	}

	row.prefui-prop, list.prefui-section {
		border: none;
		background: none;
	}

	.prefui-section {
		margin:  0 10px;
		padding: 0;
	}

	.prefui-section > row {
		margin: 8px 4px;
	}

	.prefui-section > row,
	.prefui-section > row:hover,
	.prefui-section > row:active {
		background: none;
	}

	.prefui-prop > box.vertical > .prefui-prop {
		margin-top: 6px;
	}

	.prefui-prop > box.horizontal > .prefui-prop {
		margin-left: 6px;
	}

	.prefui-prop-description {
		font-size: 0.9em;
		color: mix(@theme_fg_color, @theme_bg_color, 0.15);
	}

	.prefui-prop-string {
		font-size: 0.9em;
	}
`)

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
	d.saver.Widget = (*dialogSaver)(&d)
	// Computers are just way too fast. Ensure that the loading circle visibly
	// pops up before it closes.
	d.saver.Minimum = 100 * time.Millisecond

	d.box = gtk.NewBox(gtk.OrientationVertical, 0)

	sections := prefs.ListProperties(ctx)
	d.sections = make([]*section, len(sections))
	for i, section := range sections {
		d.sections[i] = newSection(&d, section)
		d.box.Append(d.sections[i])
	}

	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetVExpand(true)
	scroll.SetChild(d.box)

	searchEntry := gtk.NewSearchEntry()
	searchEntry.SetObjectProperty("placeholder-text", locale.Get("Search Preferences..."))
	searchEntry.ConnectSearchChanged(func() { d.Search(searchEntry.Text()) })

	d.search = gtk.NewSearchBar()
	d.search.SetChild(searchEntry)
	d.search.ConnectEntry(&searchEntry.EditableTextWidget)

	searchButton := gtk.NewToggleButton()
	searchButton.SetIconName("system-search-symbolic")
	searchButton.ConnectClicked(func() {
		d.search.SetSearchMode(searchButton.Active())
	})
	d.search.NotifyProperty("search-mode-enabled", func() {
		searchButton.SetActive(d.search.SearchMode())
	})

	outerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	outerBox.Append(d.search)
	outerBox.Append(scroll)

	d.Dialog = gtk.NewDialogWithFlags(
		locale.Get("Preferences"), app.GTKWindowFromContext(ctx),
		gtk.DialogDestroyWithParent|gtk.DialogUseHeaderBar,
	)
	d.Dialog.AddCSSClass("prefui-dialog")
	d.Dialog.SetTransientFor(app.GTKWindowFromContext(ctx))
	d.Dialog.SetModal(true)
	d.Dialog.SetDefaultSize(400, 500)
	d.Dialog.SetChild(outerBox)

	if app.IsDevel() {
		d.Dialog.AddCSSClass("devel")
	}

	// Set this to the whole dialog instead of just the child.
	d.search.SetKeyCaptureWidget(d.Dialog)

	d.loading = gtk.NewSpinner()
	d.loading.SetSizeRequest(24, 24)
	d.loading.Hide()

	d.header = d.Dialog.HeaderBar()
	d.header.PackEnd(searchButton)
	d.header.PackEnd(d.loading)

	return &d
}

func (d *Dialog) Search(query string) {
	query = strings.ToLower(query)
	for _, section := range d.sections {
		section.Search(query)
	}
}

func (d *Dialog) save() {
	d.saver.Save()
}

type dialogSaver Dialog

func (d *dialogSaver) SaveBegin() {
	d.loading.Show()
	d.loading.Start()
}

func (d *dialogSaver) SaveEnd() {
	d.loading.Stop()
	d.loading.Hide()
}

type section struct {
	*gtk.Box
	name *gtk.Label
	list *gtk.ListBox

	props []*propRow

	searching string
	noResults bool
}

func newSection(d *Dialog, sect prefs.ListedSection) *section {
	s := section{}
	s.list = gtk.NewListBox()
	s.list.AddCSSClass("prefui-section")
	s.list.SetSelectionMode(gtk.SelectionNone)
	s.list.SetActivateOnSingleClick(true)

	s.props = make([]*propRow, len(sect.Props))
	for i, prop := range sect.Props {
		s.props[i] = newPropRow(d, prop)
		s.list.Append(s.props[i])
	}

	s.name = gtk.NewLabel(sect.Name)
	s.name.AddCSSClass("prefui-heading")
	s.name.SetXAlign(0)

	s.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	s.Box.AddCSSClass("prefui-section-box")
	s.Box.Append(s.name)
	s.Box.Append(s.list)

	s.list.SetFilterFunc(func(row *gtk.ListBoxRow) bool {
		prop := s.props[row.Index()]

		if strings.Contains(prop.queryTerm, s.searching) {
			s.noResults = false
			return true
		}

		return false
	})

	return &s
}

func (s *section) Search(query string) {
	s.noResults = true
	s.searching = query
	s.list.InvalidateFilter()
	// Hide if no results.
	s.SetVisible(!s.noResults)
}

type propRow struct {
	*gtk.ListBoxRow
	box *gtk.Box

	left struct {
		*gtk.Box
		name *gtk.Label
		desc *gtk.Label
	}
	action propWidget

	queryTerm string
}

type propWidget struct {
	gtk.Widgetter
	long bool
}

func newPropRow(d *Dialog, prop prefs.LocalizedProp) *propRow {
	row := propRow{
		// Hacky way to do case-insensitive search.
		queryTerm: strings.ToLower(prop.Name) + strings.ToLower(prop.Description),
	}

	row.ListBoxRow = gtk.NewListBoxRow()
	row.AddCSSClass("prefui-prop")

	row.left.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	row.left.SetHExpand(true)

	row.left.name = gtk.NewLabel(prop.Name)
	row.left.name.AddCSSClass("prefui-prop-name")
	row.left.name.SetUseMarkup(true)
	row.left.name.SetVExpand(true)
	row.left.name.SetXAlign(0)
	row.left.name.SetWrap(true)
	row.left.name.SetWrapMode(pango.WrapWordChar)
	row.left.Append(row.left.name)

	if prop.Description != "" {
		row.left.desc = gtk.NewLabel(prop.Description)
		row.left.desc.AddCSSClass("prefui-prop-description")
		row.left.desc.SetUseMarkup(true)
		row.left.desc.SetXAlign(0)
		row.left.desc.SetWrap(true)
		row.left.desc.SetWrapMode(pango.WrapWordChar)
		row.left.Append(row.left.desc)
	}

	row.action = propWidget{
		Widgetter: prop.CreateWidget(d.ctx, d.save),
		long:      prop.WidgetIsLarge(),
	}
	gtk.BaseWidget(row.action).SetVAlign(gtk.AlignCenter)

	orientation := gtk.OrientationHorizontal
	if row.action.long {
		orientation = gtk.OrientationVertical
	}

	row.box = gtk.NewBox(orientation, 0)
	row.box.Append(row.left)
	row.box.Append(row.action)

	row.SetChild(row.box)

	return &row
}

func (r *propRow) Activate() bool {
	return gtk.BaseWidget(r.action).Activate()
}
