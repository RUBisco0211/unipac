package views

import (
	"slices"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// searchView searches the adapters that advertise the Search capability and
// lists what they return.
type searchView struct {
	data      *data
	submit    func(query string, managerIDs []string)
	onInstall func(manager.Package, actionOrigin)

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

	// seenManagers is the manager list the results were last checked against.
	seenManagers int
}

func (s *searchView) view(c *ui.Context) {
	s.syncManagers()
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Column(c).Shrink(0).Gap(12).Padding(0, 16, 0, 16).Children(func() {
			s.toolbar(c)
			s.chips(c)
		})
		// The bar sits over the body, whose slot is always reserved, so the
		// placeholder or results do not move when a search starts or ends.
		ui.Column(c).Fill().MinHeight(0).Children(func() {
			s.body(c)
			if s.searching {
				loadingBar(c).Label(locales.SearchProgress())
			}
		})
	})
}

// syncManagers drops results from managers that were turned off since the
// search ran, so an install button cannot outlive the setting that allowed it.
func (s *searchView) syncManagers() {
	if s.seenManagers == s.data.managersVersion && s.filterReady {
		return
	}
	s.seenManagers = s.data.managersVersion
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
func (s *searchView) resetManagerFilter() {
	s.managerFilter = make(map[string]bool)
	for _, info := range s.searchableManagers() {
		s.managerFilter[info.ID] = true
	}
	s.managerAll = true
	s.filterReady = true
}

func (s *searchView) searchableManagers() []manager.Info {
	out := make([]manager.Info, 0, len(s.data.managers))
	for _, info := range s.data.managers {
		if info.Enabled && info.Capabilities.Search {
			out = append(out, info)
		}
	}
	return out
}

func (s *searchView) allManagersSelected() bool {
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
func (s *searchView) selectedManagerIDs() []string {
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

func (s *searchView) managerFilterLabel() string {
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
func (s *searchView) rerunSearch() {
	query := strings.TrimSpace(s.activeQuery)
	if query == "" || s.submit == nil {
		return
	}
	s.submit(query, s.selectedManagerIDs())
}

// enabledResults keeps the packages whose manager is still in use. A search can
// return after a manager was turned off, so this also runs when results land,
// not only when the manager list changes.
func (s *searchView) enabledResults(results []manager.Package) []manager.Package {
	kept := results[:0]
	for _, pkg := range results {
		if s.data.enabled(pkg.Manager) {
			kept = append(kept, pkg)
		}
	}
	return kept
}

func (s *searchView) toolbar(c *ui.Context) {
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
		if (search.Clicked() || box.Submitted()) && s.submit != nil {
			query := strings.TrimSpace(s.query)
			s.activeQuery = query
			s.submit(query, s.selectedManagerIDs())
		}
	})
}

// managerSelect is a multi-select: the popover's checkboxes decide which
// managers the backend searches.
func (s *searchView) managerSelect(c *ui.Context) {
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

func (s *searchView) chips(c *ui.Context) {
	if s.err != nil {
		components.Text(c, locales.SearchError(s.err.Error())).TextColor(c.Theme().Danger)
	}
}

func (s *searchView) columns() []ui.TableColumn {
	columns := searchFields()
	return append(columns, ui.TableColumn{
		Title: locales.TableActions(), ID: "actions", Width: 64, Fixed: true, Align: ui.End,
	})
}

func (s *searchView) applySort() {
	col := s.sortOrder.Column
	desc := s.sortOrder.Descending
	slices.SortStableFunc(s.results, func(a, b manager.Package) int {
		var order int
		switch col {
		case "type":
			order = strings.Compare(typeLabel(a.IsGUI), typeLabel(b.IsGUI))
		case "manager":
			order = strings.Compare(
				strings.ToLower(s.data.managerName(a.Manager)),
				strings.ToLower(s.data.managerName(b.Manager)),
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

func (s *searchView) body(c *ui.Context) {
	switch {
	case s.searching:
		placeholder(c, locales.SearchSearching())
	case !s.ran:
		placeholder(c, locales.SearchHint())
	case len(s.results) == 0:
		// Name the query the results belong to, not what is typed now.
		placeholder(c, locales.SearchNoResults(s.resultQuery))
	default:
		s.table.Sort = &s.sortOrder
		s.table.OnSort = func() { s.applySort() }
		// Match the search field's left inset; the table's own first-cell
		// padding is one spacing unit tighter.
		ui.Box(c).Fill().MinHeight(0).Padding(0, 0, 0, c.Theme().Space(1)).Children(func() {
			components.TableVirtualized(c, &s.table, s.columns(), len(s.results), func(row, col int) {
				pkg := s.results[row]
				if col < len(searchFieldIndex) {
					packageFieldCell(c, pkg, searchFieldIndex[col], s.data.managerName)
					return
				}
				s.rowAction(c, pkg)
			}).Fill()
		})
	}
}

func (s *searchView) rowAction(c *ui.Context, pkg manager.Package) {
	caps := s.data.capabilities(pkg.Manager)
	running := s.data.actionFor(pkg)
	installing := running == "install"
	canInstall := caps.Install && running == ""
	label := locales.ActionsInstall()
	if installing {
		label = locales.ActionsInstalling()
	}
	slot := actionSlot(c)
	slot.Children(func() {
		install := components.IconButton(c, lucide.Download(), label, components.IconButtonPlain)
		install.Disabled(!canInstall)
		if install.Clicked() && canInstall && s.onInstall != nil {
			s.onInstall(pkg, actionPoint(install))
		}
	})
	switch {
	case installing:
		// Its own label already says it is working.
	case !s.data.enabled(pkg.Manager):
		actionTooltip(c, slot, canInstall, locales.ActionManagerOff())
	case !caps.Install:
		actionTooltip(c, slot, canInstall, locales.ActionUnsupported())
	case running != "":
		actionTooltip(c, slot, canInstall, locales.ActionWorking())
	}
}

// apply installs the outcome of a background search.
func (s *searchView) apply(query string, results []manager.Package, err error) {
	s.activeQuery = query
	s.searching = false
	s.err = err
	s.ran = true
	s.resultQuery = query
	s.results = s.enabledResults(results)
	s.seenManagers = s.data.managersVersion
	s.applySort()
}
