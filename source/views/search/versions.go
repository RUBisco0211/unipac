package search

import (
	"slices"
	"strconv"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// maxVersions is how many versions the picker lists at once. A registry keeps
// every version it ever published — npm answers thousands — and the oldest ones
// are not what a picker is for; the search above the list still reaches them.
const maxVersions = 100

// versionRows is how many versions the dialog shows without scrolling. The box
// is as tall as the list it holds, up to this many, so a package with three
// versions does not sit in a tall empty box.
const versionRows = 8

// versionRowHeight is the height of one version in the list.
const versionRowHeight = 7

// versionState is one package's version lookup: the answer, what the lookup is
// doing, and the sequence number that drops a stale one.
type versionState struct {
	// versions is the whole answer, newest first, as the backend gave it.
	versions []string
	loading  bool
	err      error
	// seq counts the lookups asked for this package. An answer about an older
	// one is dropped: a second open may have started a newer lookup.
	seq int
}

// picker is the version dialog's state: which row it is open for, what is typed
// in it, and what is chosen in it.
type picker struct {
	open bool
	pkg  manager.Package
	// key is the package the query and the choice belong to. Opening the picker
	// on another row starts clean instead of carrying the last one's.
	key    string
	query  string
	chosen string
	list   components.ListState
}

// pickable reports whether an install can offer a version at all: the manager
// advertises the lookup, and the shell wired both halves of it. A manager
// without the capability installs the newest version as it always did, with no
// control to choose another.
func (s *View) pickable(caps manager.Capabilities) bool {
	return caps.ListVersions && s.OnVersions != nil && s.OnInstallVersion != nil
}

// openVersions opens the version dialog for a package. The first open fetches
// the list; a later one reuses what arrived, because the answer changes only as
// often as the registry publishes.
func (s *View) openVersions(pkg manager.Package) {
	if key := inventory.Key(pkg); s.picker.key != key {
		s.picker.key = key
		s.picker.query = ""
		s.picker.chosen = ""
		s.picker.list.ScrollTo(0, ui.Start)
	}
	s.picker.pkg = pkg
	s.picker.open = true
	s.fetchVersions(pkg)
	s.chooseNewest()
}

// chooseNewest makes the newest version the choice while nothing is chosen, so
// the dialog opens ready to confirm: installing what the registry calls latest
// is the common case, and choosing another version is the exception. A choice
// the user already made is never replaced.
func (s *View) chooseNewest() {
	if s.picker.chosen != "" {
		return
	}
	state := s.versions[s.picker.key]
	if state == nil || len(state.versions) == 0 {
		return
	}
	s.picker.chosen = state.versions[0]
}

// fetchVersions asks the shell for a package's versions, unless the answer in
// hand is one to keep.
func (s *View) fetchVersions(pkg manager.Package) {
	seq, fetch := s.BeginVersions(pkg)
	if !fetch {
		return
	}
	s.OnVersions(pkg, seq)
}

// BeginVersions registers a lookup and reports the sequence number its answer
// must carry, and false when there is nothing to fetch: a list already in hand
// is shown again, and only a failure is retried.
func (s *View) BeginVersions(pkg manager.Package) (int, bool) {
	state := s.versionState(pkg)
	if state.loading || (state.err == nil && state.versions != nil) {
		return 0, false
	}
	state.seq++
	state.loading = true
	state.err = nil
	return state.seq, true
}

// SetVersions installs a lookup's answer, unless a newer lookup for the same
// package started since.
func (s *View) SetVersions(pkg manager.Package, seq int, versions []string, err error) {
	state := s.versionState(pkg)
	if state.seq != seq {
		return
	}
	state.loading = false
	if err != nil {
		state.err = err
		state.versions = nil
		return
	}
	state.err = nil
	state.versions = newestFirst(versions)
	if s.picker.open && s.picker.key == inventory.Key(pkg) {
		// The dialog was waiting on this answer: it opens on the newest
		// version rather than on nothing to confirm.
		s.chooseNewest()
	}
}

// versionState is a package's lookup, created empty the first time it is asked
// for.
func (s *View) versionState(pkg manager.Package) *versionState {
	if s.versions == nil {
		s.versions = make(map[string]*versionState)
	}
	key := inventory.Key(pkg)
	state, ok := s.versions[key]
	if !ok {
		state = &versionState{}
		s.versions[key] = state
	}
	return state
}

// shownVersions is what the dialog lists: the answer narrowed by the typed
// query, newest first. The whole answer is filtered before it is capped, so a
// search reaches an old version the cap would have hidden, and cut says the
// list was capped.
func (s *View) shownVersions() (shown []string, cut bool) {
	state := s.versions[s.picker.key]
	if state == nil {
		return nil, false
	}
	query := strings.ToLower(strings.TrimSpace(s.picker.query))
	matches := make([]string, 0, len(state.versions))
	for _, version := range state.versions {
		if query == "" || strings.Contains(strings.ToLower(version), query) {
			matches = append(matches, version)
		}
	}
	if len(matches) > maxVersions {
		return matches[:maxVersions], true
	}
	return matches, false
}

// versionDialog asks which version to install. It is built while it is open and
// every frame after: the lookup's answer arrives through SetVersions, which
// repaints the window.
func (s *View) versionDialog(c *ui.Context) {
	t := c.Theme()
	components.Dialog(c, &s.picker.open, components.DialogOptions{Label: locales.VersionsTitle()}, func(panel ui.Element) {
		pkg := s.picker.pkg
		state := s.versions[s.picker.key]
		loading := state == nil || state.loading
		var failure error
		if state != nil {
			failure = state.err
		}
		shown, cut := s.shownVersions()
		panel.Width(420).Padding(20).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, locales.VersionsTitle()).FontSize(16).Bold()
				components.Text(c, locales.VersionsFor(pkg.Name, s.Data.Name(pkg.Manager))).TextColor(t.TextMuted)
				components.TextBox(c, &s.picker.query, components.TextBoxOptions{LeadingIcon: lucide.Search()}).
					Placeholder(locales.VersionsSearch()).
					Label(locales.VersionsSearch()).
					AutoFocus().
					FillWidth()
				// The list takes the height of what it holds, so the dialog is the
				// same size for a hundred versions and for three.
				ui.Column(c).Shrink(0).Height(s.versionHeight(t, len(shown), cut, loading || failure != nil)).Children(func() {
					s.versionList(c, loading, failure, shown, cut)
				})
				s.versionFooter(c, pkg, loading, failure)
			})
		})
	})
}

// versionHeight is how tall the list area is: one row per version it shows, up
// to versionRows of them, and room for the note when the list was cut or for the
// line that says what the lookup is doing.
func (s *View) versionHeight(t *ui.Theme, rows int, cut, message bool) float32 {
	height := t.Space(versionRowHeight) * float32(min(rows, versionRows))
	if cut {
		height += t.Space(5.5)
	}
	if message || rows == 0 {
		// A line in the middle of an area a few rows tall reads as an answer,
		// where one in an empty strip reads as a broken list.
		height = t.Space(versionRowHeight) * 3
	}
	return height
}

// versionList is the dialog's body: what the lookup is doing, or the versions it
// answered, newest first.
func (s *View) versionList(c *ui.Context, loading bool, failure error, shown []string, cut bool) {
	t := c.Theme()
	switch {
	case loading:
		ui.Column(c).Fill().Center().Children(func() {
			components.Text(c, locales.VersionsLoading()).TextColor(t.TextMuted)
		})
	case failure != nil:
		ui.Column(c).Fill().Center().Padding(0, t.Space(2)).Children(func() {
			// The tool's own message is in the log: it is often a page of npm
			// warnings, and the line here is what the user can act on.
			components.Text(c, locales.VersionsFailed()).TextColor(t.Danger)
		})
	case len(shown) == 0:
		ui.Column(c).Fill().Center().Children(func() {
			components.Text(c, locales.VersionsEmpty()).TextColor(t.TextMuted)
		})
	default:
		ui.Column(c).Fill().Children(func() {
			if cut {
				// The list is the newest few of a longer answer, so say so
				// rather than looking like all of it.
				components.Text(c, locales.VersionsTruncated(strconv.Itoa(maxVersions))).
					FontSize(12).TextColor(t.TextMuted).Shrink(0).
					Padding(0, t.Space(2), t.Space(1.5), t.Space(2))
			}
			components.List(c, &s.picker.list, len(shown), func(row int) {
				s.versionRow(c, shown[row])
			}).Fill()
		})
	}
}

// versionRow is one version to choose. The chosen one keeps a filled face, so
// the confirm below names an obvious answer.
func (s *View) versionRow(c *ui.Context, version string) {
	t := c.Theme()
	chosen := version == s.picker.chosen
	row := components.Button(c, "", components.ButtonGhost).
		FillWidth().Height(t.Space(versionRowHeight)).
		Justify(ui.Start).AlignItems(ui.Center).Padding(0, t.Space(2)).
		Label(version)
	if chosen {
		row.Background(t.Accent.Alpha(0.14)).TextColor(t.Accent)
	}
	row.Children(func() {
		ui.Box(c).Width(16).Children(func() {
			if chosen {
				components.Icon(c, lucide.Check()).Size(14, 14)
			}
		})
		components.Text(c, version).SingleLine()
	})
	if row.Clicked() {
		s.picker.chosen = version
	}
}

// versionFooter confirms the choice. A lookup that failed still leaves the
// package installable: the button installs the newest version, which is what
// the row's install did before the picker existed.
func (s *View) versionFooter(c *ui.Context, pkg manager.Package, loading bool, failure error) {
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if components.Button(c, locales.CommonCancel(), components.ButtonOutline).Clicked() {
			s.picker.open = false
		}
		if failure != nil {
			install := components.Button(c, locales.VersionsInstallLatest(), components.ButtonPrimary)
			if install.Clicked() {
				s.installVersion(pkg, "", flight.OriginOf(install))
			}
			return
		}
		install := components.Button(c, locales.VersionsConfirm(), components.ButtonPrimary)
		install.Disabled(loading || s.picker.chosen == "")
		if install.Clicked() && !loading && s.picker.chosen != "" {
			s.installVersion(pkg, s.picker.chosen, flight.OriginOf(install))
		}
	})
}

// installVersion closes the dialog and starts the install. An empty version asks
// for the newest, the way a plain install does.
func (s *View) installVersion(pkg manager.Package, version string, at flight.Origin) {
	s.picker.open = false
	if s.OnInstallVersion != nil {
		s.OnInstallVersion(pkg, version, at)
	}
}

// newestFirst orders versions the way a picker should show them: newest first,
// and a release above its own prereleases. The adapters answer in whatever order
// their tool prints, so the page does not rely on it.
func newestFirst(versions []string) []string {
	out := slices.Clone(versions)
	slices.SortStableFunc(out, func(a, b string) int {
		return -compareVersions(a, b)
	})
	return out
}

// compareVersions orders two version strings: -1, 0 or 1. Parts compare
// numerically where both are numbers, a number sorts above a word, a version
// with a part the other lacks sorts above it (1.2.1 above 1.2), and a release
// sorts above its prereleases (1.2.0 above 1.2.0-beta.1).
func compareVersions(a, b string) int {
	aParts, aPre := splitVersion(a)
	bParts, bPre := splitVersion(b)
	for i := range min(len(aParts), len(bParts)) {
		if order := comparePart(aParts[i], bParts[i]); order != 0 {
			return order
		}
	}
	if len(aParts) != len(bParts) {
		if len(aParts) > len(bParts) {
			return 1
		}
		return -1
	}
	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "":
		return 1
	case bPre == "":
		return -1
	}
	aPrerelease, bPrerelease := strings.Split(aPre, "."), strings.Split(bPre, ".")
	for i := range min(len(aPrerelease), len(bPrerelease)) {
		if order := comparePart(aPrerelease[i], bPrerelease[i]); order != 0 {
			return order
		}
	}
	switch {
	case len(aPrerelease) == len(bPrerelease):
		return 0
	case len(aPrerelease) > len(bPrerelease):
		return 1
	default:
		return -1
	}
}

// splitVersion cuts a version into its dot-separated parts and its prerelease.
// A leading "v" and the build metadata after "+" carry no order, so both go.
func splitVersion(version string) (parts []string, prerelease string) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if i := strings.IndexByte(version, '+'); i >= 0 {
		version = version[:i]
	}
	if i := strings.IndexByte(version, '-'); i >= 0 {
		return strings.Split(version[:i], "."), version[i+1:]
	}
	return strings.Split(version, "."), ""
}

// comparePart orders one dot-separated part of a version.
func comparePart(a, b string) int {
	aNumber, aNumeric := numericPart(a)
	bNumber, bNumeric := numericPart(b)
	switch {
	case aNumeric && bNumeric:
		// Compare the digits rather than converting them, so a part longer than
		// an int still orders by value.
		if len(aNumber) != len(bNumber) {
			if len(aNumber) > len(bNumber) {
				return 1
			}
			return -1
		}
		return strings.Compare(aNumber, bNumber)
	case aNumeric:
		return 1
	case bNumeric:
		return -1
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// numericPart reports the digits of a part with their leading zeros dropped, so
// two of them compare as numbers.
func numericPart(part string) (string, bool) {
	if part == "" {
		return "", false
	}
	for _, r := range part {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	digits := strings.TrimLeft(part, "0")
	if digits == "" {
		return "0", true
	}
	return digits, true
}
