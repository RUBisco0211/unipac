package parts

import (
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// DetailPanelOptions is what a page shows in the side panel for one package: what
// it already knows, what was read about it, and the two ways a fact can be acted
// on.
type DetailPanelOptions struct {
	// Package is the row the user opened; Detail is what its own page says,
	// which arrives after the panel does.
	Package manager.Package
	Detail  manager.Detail
	// Loading says the facts are being read, so the panel says so instead of
	// showing an empty page.
	Loading bool
	// Err is why the page is incomplete, when it is: the local facts still show.
	Err string

	// OnOpenURL opens a web address, and OnReveal shows a path in the file
	// manager. Either may be nil, which hides its button.
	OnOpenURL func(string)
	OnReveal  func(string)
	// OnClose puts the panel away.
	OnClose func()
}

// DetailPanel is a package's own page beside the table: what it is, who publishes
// it, where it lives, and the way to each address it names. Rows a manager cannot
// answer show a dash rather than disappearing, so the same rows appear for every
// package.
func DetailPanel(c *ui.Context, o DetailPanelOptions) {
	t := c.Theme()
	ui.Column(c).Fill().MinWidth(0).Padding(t.Space(3)).Gap(t.Space(3)).Children(func() {
		header(c, o)
		if o.Loading {
			ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				components.Spinner(c, components.SpinnerRing)
				components.Text(c, locales.DetailLoading()).FontSize(12).TextColor(t.TextMuted)
			})
		}
		if o.Package.Description != "" && o.Detail.Summary == "" {
			components.Text(c, o.Package.Description).FontSize(12).TextColor(t.TextMuted).Wrap()
		}
		if o.Detail.Summary != "" {
			components.Text(c, o.Detail.Summary).FontSize(12).TextColor(t.TextMuted).Wrap()
		}
		ui.Column(c).FillWidth().Children(func() {
			valueRow(c, locales.DetailVersion(), func() {
				versionLine(c, o)
			})
			Divider(c)
			linkRow(c, locales.DetailHomepage(), o.Detail.Homepage, locales.DetailHomepageOpen(), o.OnOpenURL)
			Divider(c)
			valueRow(c, locales.DetailPublisher(), func() {
				components.Text(c, OrDash(o.Detail.Publisher)).FontSize(12).SingleLine()
			})
			Divider(c)
			valueRow(c, locales.DetailLicense(), func() {
				components.Text(c, OrDash(o.Detail.License)).FontSize(12).SingleLine()
			})
			Divider(c)
			linkRow(c, locales.DetailLocation(), o.Detail.Path, locales.DetailReveal(), o.OnReveal)
			Divider(c)
			valueRow(c, locales.DetailSource(), func() {
				components.Text(c, OrDash(o.Detail.Source)).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
		})
		if o.Err != "" {
			components.Text(c, locales.DetailIncomplete(o.Err)).FontSize(11).TextColor(t.TextMuted).Wrap()
		}
	})
}

// header is the package the panel is about: its name, the manager it came from,
// and the way to put the panel away.
func header(c *ui.Context, o DetailPanelOptions) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
		ManagerLogo(o.Package.Manager)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			components.Tooltip(c, components.Text(c, o.Package.Name).Bold().SingleLine(), o.Package.Name)
			// The project's own name, when the registry spells it differently:
			// "Android SDK Platform-Tools" beside the token brew installed.
			if full := o.Detail.Name; full != "" && full != o.Package.Name {
				components.Text(c, full).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
		closeButton(c, o.OnClose)
	})
}

// closeButton puts the panel away.
func closeButton(c *ui.Context, close func()) {
	if close == nil {
		return
	}
	button := components.IconButton(c, lucide.X(), locales.DetailClose(), components.IconButtonPlain).Shrink(0)
	if button.Clicked() {
		close()
	}
}

// versionLine is the installed version and, when they differ, the one the manager
// offers.
func versionLine(c *ui.Context, o DetailPanelOptions) {
	t := c.Theme()
	installed := OrDash(o.Detail.Version)
	if installed == "-" {
		installed = OrDash(o.Package.Version)
	}
	ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
		components.Text(c, installed).FontSize(12).SingleLine()
		latest := o.Detail.Latest
		if latest == "" {
			latest = o.Package.LatestVersion
		}
		if latest == "" || latest == installed {
			return
		}
		components.Text(c, "→").FontSize(12).TextColor(t.TextMuted)
		components.Text(c, latest).FontSize(12).TextColor(t.Accent).SingleLine()
	})
}

// valueRow is one fact: what it is on the leading side, the answer across the
// rest of the row, so a long value has room.
func valueRow(c *ui.Context, label string, value func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Padding(t.Space(1.5), 0).Gap(t.Space(3)).AlignItems(ui.Center).Children(func() {
		components.Text(c, label).FontSize(11).TextColor(t.TextMuted).Width(t.Space(20)).Shrink(0)
		ui.Box(c).Grow(1).MinWidth(0).Children(value)
	})
}

// linkRow is a fact that is a place: the address, and the button that opens it.
// A manager that does not answer it shows a dash and no button.
func linkRow(c *ui.Context, label, value, action string, open func(string)) {
	valueRow(c, label, func() {
		t := c.Theme()
		ui.Row(c).FillWidth().MinWidth(0).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				if value == "" {
					components.Text(c, "-").FontSize(12).TextColor(t.TextMuted)
					return
				}
				components.Tooltip(c, components.Text(c, value).FontSize(12).SingleLine(), value)
			})
			if value == "" || open == nil {
				return
			}
			button := components.Button(c, action, components.ButtonGhost).Shrink(0)
			if button.Clicked() {
				open(value)
			}
		})
	})
}
