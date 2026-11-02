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
	submit    func(query string)
	onInstall func(manager.Package)

	query       string
	resultQuery string
	seq         int
	results     []manager.Package
	searching   bool
	err         error
	ran         bool

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
		s.body(c)
	})
}

// syncManagers drops results from managers that were turned off since the
// search ran, so an install button cannot outlive the setting that allowed it.
func (s *searchView) syncManagers() {
	if s.seenManagers == s.data.managersVersion {
		return
	}
	s.seenManagers = s.data.managersVersion
	s.results = s.enabledResults(s.results)
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
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.End).Gap(8).Children(func() {
		box := components.TextBox(c, &s.query, components.TextBoxOptions{
			LeadingIcon: lucide.Search(),
		}).
			Placeholder(locales.SearchField()).
			Label(locales.SearchField()).
			Width(340)

		// A second submission is allowed while one runs: the sequence number in
		// UI.Search drops the slower, older result.
		search := components.IconButton(c, lucide.Search(), locales.SearchAction(), components.IconButtonPlain)
		search.Disabled(strings.TrimSpace(s.query) == "")
		if (search.Clicked() || box.Submitted()) && s.submit != nil {
			s.submit(strings.TrimSpace(s.query))
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
		components.TableVirtualized(c, &s.table, s.columns(), len(s.results), func(row, col int) {
			pkg := s.results[row]
			if col < len(searchFieldIndex) {
				packageFieldCell(c, pkg, searchFieldIndex[col], s.data.managerName)
				return
			}
			s.rowAction(c, pkg)
		}).Fill()
	}
}

func (s *searchView) rowAction(c *ui.Context, pkg manager.Package) {
	caps := s.data.capabilities(pkg.Manager)
	installing := s.data.busyKey == "install:"+packageKey(pkg)
	canInstall := caps.Install && !s.data.busy
	label := locales.ActionsInstall()
	if installing {
		label = locales.ActionsInstalling()
	}
	slot := actionSlot(c)
	slot.Children(func() {
		install := components.IconButton(c, lucide.Download(), label, components.IconButtonPlain)
		install.Disabled(!canInstall)
		if install.Clicked() && canInstall && s.onInstall != nil {
			s.onInstall(pkg)
		}
	})
	switch {
	case installing:
		// Its own label already says it is working.
	case !s.data.enabled(pkg.Manager):
		actionTooltip(c, slot, canInstall, locales.ActionManagerOff())
	case !caps.Install:
		actionTooltip(c, slot, canInstall, locales.ActionUnsupported())
	default:
		if s.data.busy {
			actionTooltip(c, slot, canInstall, locales.ActionWorking())
		}
	}
}

// apply installs the outcome of a background search.
func (s *searchView) apply(query string, results []manager.Package, err error) {
	s.searching = false
	s.err = err
	s.ran = true
	s.resultQuery = query
	s.results = s.enabledResults(results)
	s.seenManagers = s.data.managersVersion
	s.applySort()
}
