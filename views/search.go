package views

import (
	"slices"
	"strings"

	"github.com/RUBisco0211/unipac/backend/core/manager"
	"github.com/RUBisco0211/unipac/locales"
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

	query     string
	results   []manager.Package
	searching bool
	err       error
	ran       bool

	table     components.TableVirtualizedState
	sortOrder ui.SortOrder
}

func (s *searchView) view(c *ui.Context) {
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Column(c).Shrink(0).Gap(12).Padding(0, 16, 0, 16).Children(func() {
			s.toolbar(c)
			s.chips(c)
		})
		s.body(c)
	})
}

func (s *searchView) toolbar(c *ui.Context) {
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.End).Gap(8).Children(func() {
		box := components.TextBox(c, &s.query, components.TextBoxOptions{
			LeadingIcon: lucide.Search(),
		}).
			Placeholder(locales.SearchField()).
			Label(locales.SearchField()).
			Width(340)

		label := locales.SearchAction()
		if s.searching {
			label = locales.SearchSearching()
		}
		search := components.IconButton(c, lucide.Search(), label, components.IconButtonPlain)
		search.Disabled(s.searching || strings.TrimSpace(s.query) == "")
		if (search.Clicked() || box.Submitted()) && s.submit != nil && !s.searching {
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
	columns := packageFields()
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
			order = strings.Compare(s.data.managerName(a.Manager), s.data.managerName(b.Manager))
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
		placeholder(c, locales.SearchNoResults(s.query))
	default:
		s.table.Sort = &s.sortOrder
		s.table.OnSort = func() { s.applySort() }
		components.TableVirtualized(c, &s.table, s.columns(), len(s.results), func(row, col int) {
			pkg := s.results[row]
			if col < len(packageFields()) {
				packageFieldCell(c, pkg, col, s.data.managerName)
				return
			}
			s.rowAction(c, pkg)
		}).Fill()
	}
}

func (s *searchView) rowAction(c *ui.Context, pkg manager.Package) {
	caps := s.data.capabilities(pkg.Manager)
	canInstall := caps.Install && !s.data.busy
	install := components.IconButton(c, lucide.Download(), locales.ActionsInstall(), components.IconButtonPlain)
	install.Disabled(!canInstall)
	if install.Clicked() && canInstall && s.onInstall != nil {
		s.onInstall(pkg)
	}
}

// apply installs the outcome of a background search.
func (s *searchView) apply(results []manager.Package, err error) {
	s.searching = false
	s.err = err
	s.ran = true
	s.results = results
	s.applySort()
}
