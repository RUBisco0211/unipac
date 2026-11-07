package parts

import (
	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// PackagePage is one package's own page, shown in a dialog: the state a page
// keeps between frames, and the builder every page that lists packages calls.
// The three tables — installed, outdated and search — show the same page, so a
// reader who learns it once reads it everywhere.
type PackagePage struct {
	// Open is the dialog's state; the header's close button and Escape clear it.
	Open bool
	// pkg is the package the page is about, and what its own entry answered.
	pkg     manager.Package
	detail  manager.Detail
	loading bool
}

// Show opens the page for pkg, dropping what the previous package answered: a
// page is about one package at a time.
func (p *PackagePage) Show(pkg manager.Package) {
	p.Open = true
	p.pkg = pkg
	p.detail = manager.Detail{}
	p.loading = false
}

// Loading says the page's facts are being read, so the dialog says so instead of
// showing empty rows.
func (p *PackagePage) Loading() { p.loading = true }

// Apply installs what was read about pkg. An answer for another package is
// dropped: the dialog has moved on since. A read that failed is silent here — the
// rows a manager did not answer show their dash, and the shell has logged why —
// because a page that says "not found" under every field reads as an error the
// user must fix.
func (p *PackagePage) Apply(pkg manager.Package, detail manager.Detail, err error) {
	if p.pkg.Manager != pkg.Manager || p.pkg.Name != pkg.Name {
		return
	}
	p.loading = false
	p.detail = detail
}

// Pkg is the package the page shows, and whether it shows one at all.
func (p *PackagePage) Pkg() (manager.Package, bool) {
	if p.pkg.Name == "" {
		return manager.Package{}, false
	}
	return p.pkg, true
}

// PackagePageOptions is what the page's buttons do, which the page hosting it
// decides.
type PackagePageOptions struct {
	// OnOpenURL opens a web address, and OnReveal shows a path in the file
	// manager. Either may be nil, which makes the address plain text.
	OnOpenURL func(string)
	OnReveal  func(string)
	// ShowLocation tells whether the page is about a copy on this machine: a
	// search result is not installed here, so it has no location to show.
	ShowLocation bool
}

// Build draws the dialog. Call it every frame from the page that owns the state.
func (p *PackagePage) Build(c *ui.Context, o PackagePageOptions) {
	if p.pkg.Name == "" {
		return
	}
	t := c.Theme()
	Dialog(c, &p.Open, components.DialogOptions{Label: p.pkg.Name}, func(panel ui.Element) {
		panel.Width(t.Space(140)).Padding(t.Space(5)).Gap(t.Space(4)).Children(func() {
			header(c, p)
			summary(c, p, o)
			ui.Column(c).FillWidth().Children(func() {
				versionRow(c, p, o)
				Divider(c)
				textRow(c, locales.DetailPublisher(), p.detail.Publisher, p.loading)
				Divider(c)
				textRow(c, locales.DetailLicense(), p.detail.License, p.loading)
				Divider(c)
				homepageRow(c, p, o)
				if o.ShowLocation {
					Divider(c)
					locationRow(c, p, o)
				}
			})
		})
	})
}

// header is the package the page is about, the manager it came from, and the way
// out: a close button at the trailing edge, where a dialog's is.
func header(c *ui.Context, p *PackagePage) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Gap(t.Space(3)).AlignItems(ui.Start).Children(func() {
		ui.Image(c, ManagerLogo(p.pkg.Manager)).Size(t.Space(5), t.Space(5)).Fit(ui.Contain).Radius(t.Space(1)).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			components.Tooltip(c, components.Text(c, p.pkg.Name).FontSize(15).Bold().SingleLine(), p.pkg.Name)
			// The project's own name, when the registry spells it differently:
			// "Android SDK Platform-Tools" beside the token brew installed.
			if full := p.detail.Name; full != "" && full != p.pkg.Name {
				components.Text(c, full).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
			}
		})
		close := components.IconButton(c, lucide.X(), locales.DetailClose(), components.IconButtonPlain).Shrink(0)
		if close.Clicked() {
			p.Open = false
		}
	})
}

// summary is the project's own line. While the facts are being read, the block
// pulses in the shape the lines will take, so nothing jumps when they arrive.
func summary(c *ui.Context, p *PackagePage, o PackagePageOptions) {
	t := c.Theme()
	if p.loading && p.detail.Summary == "" && p.pkg.Description == "" {
		components.Skeleton(c, components.SkeletonOptions{Lines: 2, Label: locales.DetailLoading()})
		return
	}
	text := p.detail.Summary
	if text == "" {
		text = p.pkg.Description
	}
	if text != "" {
		components.Text(c, text).FontSize(12).TextColor(t.TextMuted).Wrap()
	}
}

// versionRow is the installed version and, when the manager offers another, the
// one it offers. A search result is not installed here, so it shows the version it
// can be installed at and no installed one.
func versionRow(c *ui.Context, p *PackagePage, o PackagePageOptions) {
	installed := p.detail.Version
	if installed == "" {
		installed = p.pkg.Version
	}
	latest := p.detail.Latest
	if latest == "" {
		latest = p.pkg.LatestVersion
	}
	if o.ShowLocation {
		textRow(c, locales.DetailVersion(), installed, p.loading)
		if latest != "" && latest != installed {
			textRow(c, locales.DetailLatest(), latest, p.loading)
		}
		return
	}
	textRow(c, locales.DetailLatest(), latest, p.loading)
}

// homepageRow is the project's page, which is the link itself: a reader clicks the
// address, as on a web page, rather than a button beside it.
func homepageRow(c *ui.Context, p *PackagePage, o PackagePageOptions) {
	linkRow(c, locales.DetailHomepage(), p.detail.Homepage, p.loading, o.OnOpenURL)
}

// locationRow is the copy on this machine, which asks the file manager to show it.
func locationRow(c *ui.Context, p *PackagePage, o PackagePageOptions) {
	linkRow(c, locales.DetailLocation(), p.detail.Path, p.loading, o.OnReveal)
}

// textRow is one fact: what it is on the leading side, the answer across the rest.
// An empty answer is a dash, so every package shows the same rows; while the facts
// are still being read, the answer's own shape pulses in the space it will fill.
func textRow(c *ui.Context, label, value string, loading bool) {
	row(c, label, func(c *ui.Context) {
		t := c.Theme()
		if value != "" {
			components.Tooltip(c, components.Text(c, value).FontSize(12).SingleLine(), value)
			return
		}
		if loading {
			placeholder(c, t.Space(24))
			return
		}
		components.Text(c, "-").FontSize(12).TextColor(t.TextMuted)
	})
}

// linkRow is a fact that is a place: the address itself is the control, styled as
// a link, with a tooltip carrying the whole of it.
func linkRow(c *ui.Context, label, value string, loading bool, open func(string)) {
	row(c, label, func(c *ui.Context) {
		t := c.Theme()
		if value == "" {
			if loading {
				placeholder(c, t.Space(32))
				return
			}
			components.Text(c, "-").FontSize(12).TextColor(t.TextMuted)
			return
		}
		if open == nil {
			components.Tooltip(c, components.Text(c, value).FontSize(12).SingleLine(), value)
			return
		}
		// A link is a text node: its visible words are its address, which is what
		// a reader wants to see and what a screen reader reads.
		link := components.Button(c, value, components.ButtonLink)
		if link.Clicked() {
			open(value)
		}
	})
}

// row is one fact's line: its label on the leading side and the answer across the
// rest, so a long path has room.
func row(c *ui.Context, label string, value func(*ui.Context)) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Padding(t.Space(1.5), 0).Gap(t.Space(4)).AlignItems(ui.Center).Children(func() {
		components.Text(c, label).FontSize(11.5).TextColor(t.TextMuted).Width(t.Space(22)).Shrink(0)
		ui.Box(c).Grow(1).MinWidth(0).Children(func() { value(c) })
	})
}

// placeholder is the bar a value will arrive in: the field's own shape, so the
// rows do not move when the answer does.
func placeholder(c *ui.Context, width float32) {
	t := c.Theme()
	ui.Box(c).Height(t.Space(2.5)).Width(width).Radius(t.Space(1)).Background(t.Text.Alpha(.07))
}
