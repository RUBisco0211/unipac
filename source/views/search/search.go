// Package search is the search page: it asks the adapters that advertise search
// for packages and lists what they return.
package search

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// View searches the adapters that advertise the Search capability and
// lists what they return.
type View struct {
	Data      *inventory.Data
	Submit    func(query string, managerIDs []string)
	OnInstall func(manager.Package, flight.Origin)
	OnUpdate  func(manager.Package, flight.Origin)
	// OnInstallVersion installs one version of a package, the one chosen in the
	// version dialog.
	OnInstallVersion func(manager.Package, string, flight.Origin)
	// OnVersions asks the shell for a package's versions. The page passes the
	// sequence number it registered, and the answer comes back to SetVersions.
	OnVersions func(manager.Package, int)
	// TitleActions builds the buttons this page puts in the title bar.
	TitleActions func(*ui.Context)

	query       string
	resultQuery string
	// activeQuery is the last query sent to the backend, including one still in
	// flight. A manager change reruns this query, not the query that produced
	// the visible results.
	activeQuery string
	seq         int
	results     []manager.Package
	searching   bool
	err         error
	ran         bool
	// truncated says a manager had more matches than it could give.
	truncated bool

	// managerFilter is the set of enabled, searchable managers to ask. The
	// backend receives the selection; results are never narrowed here.
	managerFilter map[string]bool
	managersOpen  bool
	filterReady   bool
	// managerAll says the last filter change meant "every searchable manager",
	// so a manager enabled later joins it.
	managerAll bool

	table     components.TableVirtualizedState
	sortOrder ui.SortOrder
	// actions is the column the row's install button is built in, worked out
	// where the columns are built.
	actions int

	// seenManagers is the manager list the results were last checked against.
	seenManagers int

	// versions caches each package's version lookup for the page's life, and
	// picker is the dialog that chooses one of them.
	versions map[string]*versionState
	picker   picker
}

func (s *View) Build(c *ui.Context) {
	s.syncManagers()
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Column(c).Shrink(0).Gap(12).Padding(0, c.Theme().Space(4), 0, c.Theme().Space(4)).Children(func() {
			s.toolbar(c)
			s.chips(c)
		})
		// The bar is painted on the body's own box, so it spans the results and
		// takes no room: the placeholder or results do not move when a search
		// starts or ends.
		body := ui.Column(c).Fill().MinHeight(0)
		body.Children(func() { s.body(c) })
		if s.searching {
			body.Label(locales.SearchProgress())
			parts.LoadingBar(c, body)
		}
	})
	s.versionDialog(c)
}

// syncManagers drops results from managers that were turned off since the
// search ran, so an install button cannot outlive the setting that allowed it.
func (s *View) syncManagers() {
	if s.seenManagers == s.Data.ManagersVersion && s.filterReady {
		return
	}
	s.seenManagers = s.Data.ManagersVersion
	s.results = s.enabledResults(s.results)
	if !s.filterReady {
		s.resetManagerFilter()
		return
	}
	// Keep choices for managers that can still search. A newly searchable
	// manager follows "all" only; a narrowed selection stays narrowed.
	previousAll := s.managerAll
	next := make(map[string]bool)
	for _, info := range s.searchableManagers() {
		if previousAll || s.managerFilter[info.ID] {
			next[info.ID] = true
		}
	}
	s.managerFilter = next
	s.managerAll = s.allManagersSelected()
}

// resetManagerFilter starts with every manager that can search, which is what
// "All managers" means; the user unchecks to narrow the backend query.
func (s *View) resetManagerFilter() {
	s.managerFilter = make(map[string]bool)
	for _, info := range s.searchableManagers() {
		s.managerFilter[info.ID] = true
	}
	s.managerAll = true
	s.filterReady = true
}

func (s *View) searchableManagers() []manager.Info {
	out := make([]manager.Info, 0, len(s.Data.Managers))
	for _, info := range s.Data.Managers {
		if info.Enabled && info.Capabilities.Search {
			out = append(out, info)
		}
	}
	return out
}

func (s *View) allManagersSelected() bool {
	managers := s.searchableManagers()
	if len(managers) == 0 {
		return false
	}
	for _, info := range managers {
		if !s.managerFilter[info.ID] {
			return false
		}
	}
	return true
}

// selectedManagerIDs is nil for "all enabled managers" and a non-nil, possibly
// empty slice when the user narrowed the set. The registry tells the two apart.
func (s *View) selectedManagerIDs() []string {
	managers := s.searchableManagers()
	if s.allManagersSelected() {
		return nil
	}
	ids := make([]string, 0, len(managers))
	for _, info := range managers {
		if s.managerFilter[info.ID] {
			ids = append(ids, info.ID)
		}
	}
	return ids
}

func (s *View) managerFilterLabel() string {
	managers := s.searchableManagers()
	selected := 0
	only := ""
	for _, info := range managers {
		if s.managerFilter[info.ID] {
			selected++
			only = info.Name
		}
	}
	switch {
	case len(managers) > 0 && selected == len(managers):
		return locales.PackagesAllManagers()
	case selected == 1:
		return only
	default:
		return locales.SearchManagerCount(selected)
	}
}

// rerunSearch asks the backend again with the new manager set, for the last
// query submitted, whether its results have arrived or not.
func (s *View) rerunSearch() {
	query := strings.TrimSpace(s.activeQuery)
	if query == "" || s.Submit == nil {
		return
	}
	s.Submit(query, s.selectedManagerIDs())
}

// enabledResults keeps the packages whose manager is still in use. A search can
// return after a manager was turned off, so this also runs when results land,
// not only when the manager list changes.
func (s *View) enabledResults(results []manager.Package) []manager.Package {
	kept := results[:0]
	for _, pkg := range results {
		if s.Data.Enabled(pkg.Manager) {
			kept = append(kept, pkg)
		}
	}
	return kept
}

func (s *View) toolbar(c *ui.Context) {
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
		box := components.TextBox(c, &s.query, components.TextBoxOptions{
			LeadingIcon: lucide.Search(),
		}).
			Placeholder(locales.SearchField()).
			Label(locales.SearchField()).
			Width(340)

		s.managerSelect(c)

		// A second submission is allowed while one runs: the sequence number in
		// UI.Search drops the slower, older result.
		search := components.IconButton(c, lucide.Search(), locales.SearchAction(), components.IconButtonPlain)
		search.Disabled(strings.TrimSpace(s.query) == "")
		if (search.Clicked() || box.Submitted()) && s.Submit != nil {
			query := strings.TrimSpace(s.query)
			s.activeQuery = query
			s.Submit(query, s.selectedManagerIDs())
		}
	})
}

// managerSelect is a multi-select: the popover's checkboxes decide which
// managers the backend searches.
func (s *View) managerSelect(c *ui.Context) {
	t := c.Theme()
	trigger := components.Button(c, s.managerFilterLabel(), components.ButtonOutline).
		Width(t.Space(45))
	trigger.Label(locales.SearchManagers())
	if trigger.Clicked() {
		s.managersOpen = !s.managersOpen
	}
	components.Popover(c, trigger, &s.managersOpen, func(panel ui.Element) {
		panel.Width(t.Space(55))
		components.Text(c, locales.SearchManagers()).FontSize(12).Bold()
		all := s.allManagersSelected()
		allBox := components.Checkbox(c, &all, locales.PackagesAllManagers())
		if allBox.Changed() {
			for _, info := range s.searchableManagers() {
				s.managerFilter[info.ID] = all
			}
			s.managerAll = all
			s.rerunSearch()
		}
		for _, info := range s.searchableManagers() {
			selected := s.managerFilter[info.ID]
			box := components.Checkbox(c, &selected, info.Name)
			if box.Changed() {
				s.managerFilter[info.ID] = selected
				s.managerAll = s.allManagersSelected()
				s.rerunSearch()
			}
		}
	})
}

func (s *View) chips(c *ui.Context) {
	if s.err != nil {
		components.Text(c, locales.SearchError(s.err.Error())).TextColor(c.Theme().Danger)
	}
}

func (s *View) columns() []ui.TableColumn {
	columns := parts.SearchFields()
	s.actions = len(columns)
	columns = append(columns, ui.TableColumn{
		Title: locales.TableActions(), ID: "actions", Width: 88, Fixed: true, Align: ui.Center,
	})
	return columns
}

func (s *View) applySort() {
	col := s.sortOrder.Column
	desc := s.sortOrder.Descending
	slices.SortStableFunc(s.results, func(a, b manager.Package) int {
		var order int
		switch col {
		case "manager":
			order = strings.Compare(
				strings.ToLower(s.Data.Name(a.Manager)),
				strings.ToLower(s.Data.Name(b.Manager)),
			)
		default:
			order = strings.Compare(a.Name, b.Name)
		}
		if order == 0 {
			order = strings.Compare(a.Name, b.Name)
		}
		if desc {
			return -order
		}
		return order
	})
}

func (s *View) body(c *ui.Context) {
	switch {
	case s.searching:
		parts.Placeholder(c, locales.SearchSearching())
	case !s.ran:
		parts.Placeholder(c, locales.SearchHint())
	case len(s.results) == 0:
		// Name the query the results belong to, not what is typed now.
		parts.Placeholder(c, locales.SearchNoResults(s.resultQuery))
	default:
		s.table.Sort = &s.sortOrder
		s.table.OnSort = func() { s.applySort() }
		ui.Column(c).Fill().MinHeight(0).Children(func() {
			if s.truncated {
				// The list is what a manager had to give within its limit, so say
				// so rather than looking like the whole answer.
				components.Text(c, locales.SearchTruncated(strconv.Itoa(manager.MaxSearchResults))).
					FontSize(12).TextColor(c.Theme().TextMuted).Shrink(0).Padding(0, c.Theme().Space(4), c.Theme().Space(1.5), c.Theme().Space(4))
			}
			// The same insets as the package tables: the table brings its own
			// cell padding, and the trailing pad column keeps the last control
			// off the window edge.
			// The same padding either side as the package tables, for the
			// header's first cell and the rows together.
			ui.Box(c).Fill().MinHeight(0).Padding(0, c.Theme().Space(2), 0, c.Theme().Space(2)).Children(func() {
				components.TableVirtualized(c, &s.table, s.columns(), len(s.results), func(row, col int) {
					pkg := s.results[row]
					switch {
					case col < len(parts.SearchFieldIndex):
						parts.Cell(c, pkg, parts.SearchFieldIndex[col], s.Data.Name, s.Data.SearchMark(pkg))
					case col == s.actions:
						s.rowAction(c, pkg)
					}
				}).Fill()
			})
		})
	}
}

// rowAction is the install button, or the update one for a package the
// inventory holds at another version: the row already says which it is.
func (s *View) rowAction(c *ui.Context, pkg manager.Package) {
	caps := s.Data.Capabilities(pkg.Manager)
	mark := s.Data.SearchMark(pkg)
	updating := mark == inventory.MarkUpdate
	running := s.Data.Running(pkg)
	working := running != "" && running == actionName(updating)
	canAct := running == "" && capability(caps, updating)
	slot := parts.ActionSlot(c)
	slot.Children(func() {
		button := s.actionButton(c, updating, working)
		button.Disabled(!canAct)
		if button.Clicked() && canAct {
			s.install(pkg, updating, flight.OriginOf(button), caps)
		}
	})
	switch {
	case working:
		// Its own label already says it is working.
	case !s.Data.Enabled(pkg.Manager):
		parts.ActionTooltip(c, slot, canAct, locales.ActionManagerOff())
	case !capability(caps, updating):
		parts.ActionTooltip(c, slot, canAct, locales.ActionUnsupported())
	case running != "":
		parts.ActionTooltip(c, slot, canAct, locales.ActionWorking())
	}
}

// actionButton is the row's button: an arrow down to install, an arrow up to
// update, and a turning loader while either runs.
func (s *View) actionButton(c *ui.Context, updating, working bool) ui.Element {
	switch {
	case updating && working:
		return spinning(c, components.IconButton(c, lucide.Loader(), locales.ActionsUpdating(), components.IconButtonPlain))
	case updating:
		return components.IconButton(c, lucide.CircleArrowUp(), locales.ActionsUpdate(), components.IconButtonPlain)
	case working:
		return spinning(c, components.IconButton(c, lucide.Loader(), locales.ActionsInstalling(), components.IconButtonPlain))
	default:
		return components.IconButton(c, lucide.Download(), locales.ActionsInstall(), components.IconButtonPlain)
	}
}

// spinning turns a button whose icon says it is working. The button is a circle
// around a glyph, so rotating it reads as the glyph turning, and its own hover
// and disabled faces come along; a button that is working is disabled, so the
// turn costs nothing the pointer would use. It holds still when the desktop asks
// for less motion.
func spinning(c *ui.Context, button ui.Element) ui.Element {
	if c.Preferences().ReduceMotion {
		return button
	}
	return button.Rotate(button.Loop("busy", 900*time.Millisecond, ui.Linear) * 360)
}

// run runs what the row's action does.
func (s *View) run(pkg manager.Package, updating bool, at flight.Origin) {
	if updating {
		if s.OnUpdate != nil {
			s.OnUpdate(pkg, at)
		}
		return
	}
	if s.OnInstall != nil {
		s.OnInstall(pkg, at)
	}
}

// install runs the row's install, which is a choice of version for a manager
// that can list them and a plain install for one that cannot.
func (s *View) install(pkg manager.Package, updating bool, at flight.Origin, caps manager.Capabilities) {
	if !updating && s.pickable(caps) {
		s.openVersions(pkg)
		return
	}
	s.run(pkg, updating, at)
}

// actionName is the operation the inventory marks a package busy with.
func actionName(updating bool) string {
	if updating {
		return "update"
	}
	return "install"
}

// capability reports what a manager claims for the action the row offers.
func capability(caps manager.Capabilities, updating bool) bool {
	if updating {
		return caps.Update
	}
	return caps.Install
}

// BeginQuery registers a new search and returns its sequence number. The caller
// passes that number back to Apply, so a slower earlier result cannot replace a
// newer one.
func (s *View) BeginQuery() int {
	s.seq++
	s.searching = true
	s.err = nil
	return s.seq
}

// Apply installs the outcome of the search started by BeginQuery, unless a newer
// query has started since.
func (s *View) Apply(seq int, query string, result manager.SearchResult, err error) {
	if seq != s.seq {
		return
	}
	s.apply(query, result, err)
}

// apply installs the outcome of a background search.
func (s *View) apply(query string, result manager.SearchResult, err error) {
	s.activeQuery = query
	s.searching = false
	s.err = err
	s.ran = true
	s.resultQuery = query
	s.truncated = result.Truncated
	s.results = s.enabledResults(result.Packages)
	// Do not mark the manager list as seen here: a manager change may have
	// happened while the search was in flight, and the next view pass must
	// reconcile the filter against the new list.
	s.applySort()
}
