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
	err     string
}

// Show opens the page for pkg, dropping what the previous package answered: a
// page is about one package at a time.
func (p *PackagePage) Show(pkg manager.Package) {
	p.Open = true
	p.pkg = pkg
	p.detail = manager.Detail{}
	p.err = ""
	p.loading = false
}

// Loading says the page's facts are being read, so the dialog says so instead of
// showing empty rows.
func (p *PackagePage) Loading() { p.loading = true }

// Apply installs what was read about pkg. An answer for another package is
// dropped: the dialog has moved on since.
func (p *PackagePage) Apply(pkg manager.Package, detail manager.Detail, err error) {
	if p.pkg.Manager != pkg.Manager || p.pkg.Name != pkg.Name {
		return
	}
	p.loading = false
	p.detail = detail
	p.err = ""
	if err != nil {
		p.err = err.Error()
	}
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
	// manager. Either may be nil, which hides its button.
	OnOpenURL func(string)
	OnReveal  func(string)
}

// Build draws the dialog. Call it every frame from the page that owns the state.
func (p *PackagePage) Build(c *ui.Context, o PackagePageOptions) {
	if p.pkg.Name == "" {
		return
	}
	t := c.Theme()
	components.Dialog(c, &p.Open, components.DialogOptions{Label: p.pkg.Name}, func(panel ui.Element) {
		panel.Width(t.Space(140)).Padding(t.Space(5)).Gap(t.Space(4)).Children(func() {
			header(c, p)
			summary(c, p)
			ui.Column(c).FillWidth().Children(func() {
				versionRow(c, p)
				Divider(c)
				valueRow(c, locales.DetailPublisher(), p.detail.Publisher, false)
				Divider(c)
				valueRow(c, locales.DetailLicense(), p.detail.License, false)
				Divider(c)
				valueRow(c, locales.DetailHomepage(), p.detail.Homepage, isURL(p.detail.Homepage))
				Divider(c)
				valueRow(c, locales.DetailLocation(), p.detail.Path, false)
			})
			if p.err != "" {
				components.Text(c, locales.DetailIncomplete(p.err)).
					FontSize(11).TextColor(t.TextMuted).Wrap()
			}
			footer(c, p, o)
		})
	})
}

// header is the package the page is about and the manager it came from.
func header(c *ui.Context, p *PackagePage) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Gap(t.Space(3)).AlignItems(ui.Center).Children(func() {
		ui.Image(c, ManagerLogo(p.pkg.Manager)).Size(t.Space(5), t.Space(5)).Fit(ui.Contain).Radius(t.Space(1)).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			components.Tooltip(c, components.Text(c, p.pkg.Name).FontSize(15).Bold().SingleLine(), p.pkg.Name)
			// The project's own name, when the registry spells it differently:
			// "Android SDK Platform-Tools" beside the token brew installed.
			if full := p.detail.Name; full != "" && full != p.pkg.Name {
				components.Text(c, full).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
			}
			if p.detail.Source != "" {
				components.Text(c, locales.DetailSourceLine(p.detail.Source)).
					FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
	})
}

// summary is the project's own line, and while the facts are being read, what the
// table already knew.
func summary(c *ui.Context, p *PackagePage) {
	t := c.Theme()
	if p.loading {
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			components.Spinner(c, components.SpinnerRing).Size(t.Space(3), t.Space(3))
			components.Text(c, locales.DetailLoading()).FontSize(12).TextColor(t.TextMuted)
		})
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
// one it offers.
func versionRow(c *ui.Context, p *PackagePage) {
	installed := OrDash(p.detail.Version)
	if installed == "-" {
		installed = OrDash(p.pkg.Version)
	}
	latest := p.detail.Latest
	if latest == "" {
		latest = p.pkg.LatestVersion
	}
	valueRow(c, locales.DetailVersion(), installed, false)
	if latest == "" || latest == installed || latest == "-" {
		return
	}
	valueRow(c, locales.DetailLatest(), latest, false)
}

// valueRow is one fact: what it is on the leading side, the answer across the
// rest. An empty answer is a dash, so every package shows the same rows.
func valueRow(c *ui.Context, label, value string, link bool) {
	t := c.Theme()
	ui.Row(c).FillWidth().MinWidth(0).Padding(t.Space(1.5), 0).Gap(t.Space(4)).AlignItems(ui.Center).Children(func() {
		components.Text(c, label).FontSize(11.5).TextColor(t.TextMuted).Width(t.Space(22)).Shrink(0)
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			if value == "" || value == "-" {
				components.Text(c, "-").FontSize(12).TextColor(t.TextMuted)
				return
			}
			text := components.Text(c, value).FontSize(12).SingleLine()
			if link {
				text.TextColor(t.Accent)
			}
			components.Tooltip(c, text, value)
		})
	})
}

// footer is the page's actions: the two places a fact can point at, and the way
// back to the table.
func footer(c *ui.Context, p *PackagePage, o PackagePageOptions) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(t.Space(2)).Justify(ui.End).Children(func() {
		if p.detail.Homepage != "" && o.OnOpenURL != nil {
			open := glyphButton(c, locales.DetailHomepageOpen(), components.ButtonOutline, lucide.ExternalLink())
			if open.Clicked() {
				o.OnOpenURL(p.detail.Homepage)
			}
		}
		if p.detail.Path != "" && o.OnReveal != nil {
			reveal := glyphButton(c, locales.DetailReveal(), components.ButtonOutline, lucide.FolderOpen())
			if reveal.Clicked() {
				o.OnReveal(p.detail.Path)
			}
		}
		close := components.Button(c, locales.DetailClose(), components.ButtonPrimary)
		if close.Clicked() {
			p.Open = false
		}
	})
}

// glyphButton is a button whose label has a glyph before it: Button appends its
// children after the label without a gap of its own, so the label and the glyph
// are one child row.
func glyphButton(c *ui.Context, label string, variant components.ButtonVariant, glyph *ui.SVG) ui.Element {
	button := components.Button(c, "", variant).Label(label)
	button.Children(func() {
		ui.Row(c).Gap(c.Theme().Space(1.5)).AlignItems(ui.Center).Children(func() {
			components.Icon(c, glyph).Size(14, 14)
			components.Text(c, label).SingleLine()
		})
	})
	return button
}

// isURL reports whether a value is a web address rather than a local path.
func isURL(value string) bool {
	return len(value) > 8 && (value[:7] == "http://" || value[:8] == "https://")
}
