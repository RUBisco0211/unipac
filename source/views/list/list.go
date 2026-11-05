// Package list is the installed and outdated page: a filterable, sortable,
// multi-selectable table over the shared package inventory.
package list

import (
	"slices"
	"strconv"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/RUBisco0211/unipac/source/views/flight"
	"github.com/RUBisco0211/unipac/source/views/inventory"
	"github.com/RUBisco0211/unipac/source/views/parts"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// View renders a filterable, sortable, multi-selectable table of the shared
// inventory. The same type backs the installed and the outdated page.
type View struct {
	// Exported fields are the page's wiring, set once by the window shell;
	// everything below them is the page's own state.

	Data *inventory.Data
	// OutdatedOnly restricts the table to packages with a newer version.
	OutdatedOnly bool
	// AllowRemove keeps uninstall off the outdated page, where the only
	// meaningful action is updating.
	AllowRemove  bool
	OnRefresh    func()
	OnUpdate     func(manager.Package, flight.Origin)
	OnRemove     func(manager.Package, flight.Origin)
	OnBulkUpdate func([]manager.Package, flight.Origin)
	OnBulkRemove func([]manager.Package, flight.Origin)

	query    string
	managers map[string]bool
	// managersOpen is the manager filter popover.
	managersOpen bool
	// managersSeen is the manager list version the filter was built for.
	managersSeen int
	// knownManagers is the enabled set at the previous sync. It tells whether
	// the old selection meant "all" before a new manager joins the list.
	knownManagers map[string]bool
	table         components.TableVirtualizedState
	// actions is the column the row's buttons are built in, worked out where the
	// columns are built: the select checkbox, the shared fields, then this one,
	// then the trailing pad.
	actions   int
	chosen    map[string]struct{}
	selection ui.Selection[any]
	sortOrder ui.SortOrder

	uninstallOpen   bool
	uninstallTarget manager.Package
	bulkOpen        bool
	bulkTargets     []manager.Package

	theme       *ui.Theme
	flat        *ui.Theme
	counts      map[string]int
	countsTotal int
	countsSeen  struct {
		version  int
		managers int
		query    string
	}

	rows    []manager.Package
	applied struct {
		version int
		filter  string
		query   string
	}
}

func (l *View) Build(c *ui.Context) {
	l.Sync()
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Column(c).Shrink(0).Gap(12).Padding(0, 16, 0, 16).Children(func() {
			l.toolbar(c)
			if l.Data.RefreshErr != nil {
				components.Text(c, locales.PackagesRefreshError(l.Data.RefreshErr.Error())).
					TextColor(c.Theme().Danger)
			}
		})
		// The slot is always reserved and the bar is drawn over it, so nothing
		// moves when a refresh starts or stops.
		ui.Column(c).Fill().Padding(2, 0, 0, 0).Children(func() {
			l.body(c)
			if l.Data.Refreshing {
				parts.LoadingBar(c)
			}
		})
	})
	l.uninstallDialog(c)
	l.bulkDialog(c)
}

// sync rebuilds the filtered rows when the inventory or a filter changed. It is
// pure in-memory work, safe inside a builder.
func (l *View) Sync() {
	l.syncManagerFilter()
	filter := l.filterKey()
	if l.applied.version == l.Data.Version &&
		l.applied.filter == filter &&
		l.applied.query == l.query {
		return
	}
	l.applied.version = l.Data.Version
	l.applied.filter = filter
	l.applied.query = l.query
	l.Rebuild()
}

// syncManagerFilter keeps the multi-select valid when the manager list changes:
// a turned-off manager leaves the selection, and if everything was selected the
// new enabled set is selected too.
func (l *View) syncManagerFilter() {
	if l.managers != nil && l.managersSeen == l.Data.ManagersVersion {
		return
	}
	previousAll := l.managers == nil || l.allManagersSelectedFor(l.knownManagers)
	enabled := make([]string, 0, len(l.Data.Managers))
	next := make(map[string]bool, len(l.Data.Managers))
	known := make(map[string]bool, len(l.Data.Managers))
	for _, info := range l.Data.Managers {
		if !info.Enabled {
			continue
		}
		enabled = append(enabled, info.ID)
		known[info.ID] = true
		if previousAll || l.managers[info.ID] {
			next[info.ID] = true
		}
	}
	if len(next) == 0 && !previousAll {
		// The managers that were selected went away; fall back to all of the
		// remaining ones rather than leaving the page empty.
		for _, id := range enabled {
			next[id] = true
		}
	}
	l.managers = next
	l.knownManagers = known
	l.managersSeen = l.Data.ManagersVersion
}

// allManagersSelectedFor reports whether the selection covers every manager in
// ids. It is used against the previous enabled set, before a newly enabled
// manager is added to the current one.
func (l *View) allManagersSelectedFor(ids map[string]bool) bool {
	if len(ids) == 0 {
		return false
	}
	for id := range ids {
		if !l.managers[id] {
			return false
		}
	}
	return true
}

func (l *View) allManagersSelected() bool {
	enabled := 0
	for _, info := range l.Data.Managers {
		if !info.Enabled {
			continue
		}
		enabled++
		if !l.managers[info.ID] {
			return false
		}
	}
	return enabled > 0
}

// filterKey identifies the selected managers so sync can tell a filter change
// from an unrelated rebuild.
func (l *View) filterKey() string {
	ids := make([]string, 0, len(l.managers))
	for id, on := range l.managers {
		if on {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return strings.Join(ids, ",")
}

func (l *View) Rebuild() {
	l.rows = l.rows[:0]
	for _, pkg := range l.Data.Rows {
		if l.OutdatedOnly && !pkg.Outdated {
			continue
		}
		if !l.managers[pkg.Manager] {
			continue
		}
		if !inventory.Matches(pkg, l.query) {
			continue
		}
		l.rows = append(l.rows, pkg)
	}
	l.applySort()
	l.pruneSelection()
}

// pruneSelection drops keys that the current filters hide, so a bulk action
// never touches a package the user cannot see.
func (l *View) pruneSelection() {
	visible := make(map[string]struct{}, len(l.rows))
	for _, pkg := range l.rows {
		visible[inventory.Key(pkg)] = struct{}{}
	}
	for key := range l.chosen {
		if _, seen := visible[key]; !seen {
			delete(l.chosen, key)
		}
	}
	l.syncSelection()
}

// isChosen reports whether the package is selected. Selection lives in chosen;
// the table's Selection only mirrors it so rows can be highlighted.
func (l *View) isChosen(pkg manager.Package) bool {
	_, ok := l.chosen[inventory.Key(pkg)]
	return ok
}

// choose selects or deselects one package.
func (l *View) choose(pkg manager.Package, on bool) {
	if l.chosen == nil {
		l.chosen = map[string]struct{}{}
	}
	key := inventory.Key(pkg)
	if on {
		l.chosen[key] = struct{}{}
	} else {
		delete(l.chosen, key)
	}
	l.syncSelection()
}

// syncSelection mirrors chosen into the table's selection. The table also
// selects a row on a plain click, which would clear the rest; restoring from
// chosen on every change keeps clicks inert and leaves the checkboxes in charge.
func (l *View) syncSelection() {
	l.selection.Clear()
	for key := range l.chosen {
		l.selection.Add(key)
	}
}

// selectColumnWidth is the width of the checkbox column.
const selectColumnWidth = 40

func (l *View) columns() []ui.TableColumn {
	columns := []ui.TableColumn{{Title: "", ID: "select", Width: selectColumnWidth, Fixed: true}}
	columns = append(columns, parts.Fields()...)
	l.actions = len(columns)
	columns = append(columns, ui.TableColumn{
		Title: locales.TableActions(), ID: "actions", Width: 88, Fixed: true, Align: ui.End,
	})
	// A hair of trailing space so the last header and the row buttons do not
	// touch the window edge.
	return append(columns, ui.TableColumn{Title: "", ID: "pad", Width: 12, Fixed: true})
}

func (l *View) applySort() {
	col := l.sortOrder.Column
	desc := l.sortOrder.Descending
	slices.SortStableFunc(l.rows, func(a, b manager.Package) int {
		var order int
		switch col {
		case "manager":
			order = strings.Compare(
				strings.ToLower(l.Data.Name(a.Manager)),
				strings.ToLower(l.Data.Name(b.Manager)),
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

func (l *View) selectedPackages() []manager.Package {
	selected := make([]manager.Package, 0, len(l.chosen))
	for _, pkg := range l.rows {
		if l.isChosen(pkg) {
			selected = append(selected, pkg)
		}
	}
	return selected
}

// managerSelect is the multi-select manager filter. The checkboxes are built
// from the component library rather than a native menu so they follow the
// theme, and the counts show what each manager contributes to this page.
func (l *View) managerSelect(c *ui.Context) {
	t := c.Theme()
	trigger := components.Button(c, l.managerLabel(), components.ButtonOutline).
		Width(t.Space(42.5))
	trigger.Label(locales.SearchManagers())
	if trigger.Clicked() {
		l.managersOpen = !l.managersOpen
	}
	counts, total := l.managerCounts()
	components.Popover(c, trigger, &l.managersOpen, func(panel ui.Element) {
		panel.Width(t.Space(55))
		components.Text(c, locales.SearchManagers()).FontSize(12).Bold()
		all := l.allManagersSelected()
		allLabel := locales.PackagesAllManagers() + " (" + strconv.Itoa(total) + ")"
		allBox := components.Checkbox(c, &all, allLabel)
		if allBox.Changed() {
			for _, info := range l.Data.Managers {
				if info.Enabled {
					l.managers[info.ID] = all
				}
			}
		}
		for _, info := range l.Data.Managers {
			if !info.Enabled {
				continue
			}
			selected := l.managers[info.ID]
			label := info.Name + " (" + strconv.Itoa(counts[info.ID]) + ")"
			box := components.Checkbox(c, &selected, label)
			if box.Changed() {
				l.managers[info.ID] = selected
			}
		}
	})
}

// managerCounts counts what each manager contributes to the page as filtered by
// its own scope and the query, ignoring the manager filter being chosen.
func (l *View) managerCounts() (map[string]int, int) {
	if l.counts != nil &&
		l.countsSeen.version == l.Data.Version &&
		l.countsSeen.managers == l.Data.ManagersVersion &&
		l.countsSeen.query == l.query {
		return l.counts, l.countsTotal
	}
	counts := make(map[string]int, len(l.Data.Managers))
	total := 0
	for _, pkg := range l.Data.Rows {
		if l.OutdatedOnly && !pkg.Outdated {
			continue
		}
		if !inventory.Matches(pkg, l.query) || !l.Data.Enabled(pkg.Manager) {
			continue
		}
		counts[pkg.Manager]++
		total++
	}
	l.counts, l.countsTotal = counts, total
	l.countsSeen = struct {
		version  int
		managers int
		query    string
	}{l.Data.Version, l.Data.ManagersVersion, l.query}
	return counts, total
}

// managerLabel names the current filter in the trigger.
// allManagersOff reports whether every package belongs to a manager that is
// turned off, which is why none of them can be updated.
func (l *View) allManagersOff(pkgs []manager.Package) bool {
	if len(pkgs) == 0 {
		return false
	}
	for _, pkg := range pkgs {
		if l.Data.Enabled(pkg.Manager) {
			return false
		}
	}
	return true
}

func (l *View) managerLabel() string {
	enabled, selected := 0, 0
	only := ""
	for _, info := range l.Data.Managers {
		if !info.Enabled {
			continue
		}
		enabled++
		if l.managers[info.ID] {
			selected++
			only = info.Name
		}
	}
	switch {
	case enabled > 0 && selected == enabled:
		return locales.PackagesAllManagers()
	case selected == 1:
		return only
	default:
		return locales.ManagerCount(selected)
	}
}

func (l *View) toolbar(c *ui.Context) {
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.End).Gap(8).Children(func() {
		components.TextBox(c, &l.query, components.TextBoxOptions{
			LeadingIcon: lucide.Search(),
		}).
			Placeholder(locales.PackagesSearchPlaceholder()).
			Label(locales.PackagesSearchPlaceholder()).
			Width(340)

		l.managerSelect(c)

		refreshLabel := locales.CommonRefresh()
		if l.Data.Refreshing {
			refreshLabel = locales.CommonRefreshing()
		}
		refresh := components.IconButton(c, lucide.RefreshCcw(), refreshLabel, components.IconButtonPlain)
		refresh.Disabled(l.Data.Refreshing)
		if refresh.Clicked() && l.OnRefresh != nil {
			l.OnRefresh()
		}

		selected := l.selectedPackages()
		if len(selected) == 0 {
			return
		}

		// Only packages that can actually be updated go to the manager. A mixed
		// selection would otherwise update what is already current, and a
		// package already being operated on would start twice.
		updatable := make([]manager.Package, 0, len(selected))
		for _, pkg := range selected {
			if pkg.Outdated && l.Data.Capabilities(pkg.Manager).Update &&
				l.Data.Running(pkg) == "" {
				updatable = append(updatable, pkg)
			}
		}
		canBulkUpdate := len(updatable) > 0
		updateSlot := parts.ActionSlot(c)
		updateSlot.Children(func() {
			bulkUpdate := components.IconButton(c, lucide.CircleArrowUp(), locales.PackagesBulkUpdate(), components.IconButtonPlain)
			bulkUpdate.Disabled(!canBulkUpdate)
			if bulkUpdate.Clicked() && canBulkUpdate && l.OnBulkUpdate != nil {
				l.OnBulkUpdate(updatable, flight.OriginOf(bulkUpdate))
			}
		})
		switch {
		case canBulkUpdate:
		case l.allManagersOff(selected):
			parts.ActionTooltip(c, updateSlot, canBulkUpdate, locales.ActionManagerOff())
		case l.Data.AnyRunning(selected):
			parts.ActionTooltip(c, updateSlot, canBulkUpdate, locales.ActionWorking())
		default:
			parts.ActionTooltip(c, updateSlot, canBulkUpdate, locales.ActionUpToDate())
		}

		if !l.AllowRemove {
			return
		}
		removable := make([]manager.Package, 0, len(selected))
		for _, pkg := range selected {
			if l.Data.Running(pkg) == "" {
				removable = append(removable, pkg)
			}
		}
		canBulkRemove := len(removable) > 0
		removeSlot := parts.ActionSlot(c)
		removeSlot.Children(func() {
			bulkRemove := components.IconButton(c, lucide.Trash(), locales.PackagesBulkRemove(), components.IconButtonPlain)
			bulkRemove.TextColor(c.Theme().Danger)
			bulkRemove.Disabled(!canBulkRemove)
			if bulkRemove.Clicked() && canBulkRemove {
				l.bulkTargets = removable
				l.bulkOpen = true
			}
		})
		if !canBulkRemove {
			parts.ActionTooltip(c, removeSlot, canBulkRemove, locales.ActionWorking())
		}
	})
}

// allSelected reports whether every visible row is chosen.
func (l *View) allSelected() bool {
	if len(l.rows) == 0 {
		return false
	}
	for _, pkg := range l.rows {
		if !l.isChosen(pkg) {
			return false
		}
	}
	return true
}

// toggleAll chooses every visible row, or clears the choice.
func (l *View) toggleAll() {
	if l.chosen == nil {
		l.chosen = map[string]struct{}{}
	}
	if l.allSelected() {
		clear(l.chosen)
		l.syncSelection()
		return
	}
	for _, pkg := range l.rows {
		l.chosen[inventory.Key(pkg)] = struct{}{}
	}
	l.syncSelection()
}

func (l *View) body(c *ui.Context) {
	switch {
	case l.Data.LoadErr != nil:
		parts.Placeholder(c, locales.PackagesLoadError(l.Data.LoadErr.Error()))
	case l.Data.Refreshing && len(l.rows) == 0:
		// Nothing cached yet: say what is happening instead of claiming the
		// inventory is empty.
		parts.Placeholder(c, locales.PackagesScanning())
	case len(l.rows) == 0:
		if l.OutdatedOnly {
			parts.Placeholder(c, locales.OutdatedEmpty())
			return
		}
		parts.Placeholder(c, locales.PackagesEmpty())
	default:
		l.syncSelection()
		l.table.Sort = &l.sortOrder
		l.table.OnSort = func() { l.applySort() }
		l.table.List.Selection = &l.selection
		l.table.List.OnChange = l.syncSelection
		l.table.List.Key = func(row int) any { return inventory.Key(l.rows[row]) }
		// Rows move when a column sorts them. Keeping the offset, rather than
		// the item at the top, is what stops the table from scrolling along
		// with that item to wherever the sort puts it.
		l.table.List.KeepOffset = true
		t := c.Theme()
		ui.Column(c).Fill().Children(func() {
			components.TableVirtualized(c, &l.table, l.columns(), len(l.rows), func(row, col int) {
				l.cell(c, row, col)
			}).Fill()
			// The table header only takes text, so the select-all control is
			// laid over the first header cell, on the same padding the cells use.
			ui.Row(c).Absolute().Top(0).Left(t.Space(2.5) + 6).Height(t.Space(8)).
				AlignItems(ui.Center).Children(func() {
				all := l.allSelected()
				box := components.Checkbox(c, &all, "").Label(locales.PackagesSelectAll())
				if box.Changed() {
					l.toggleAll()
				}
			})
			if len(l.chosen) > 0 {
				titleWidth, _ := c.MeasureText(0, ui.Span{Text: locales.TablePackage(), Weight: 600})
				left := selectColumnWidth + t.Space(2.5) + titleWidth + t.Space(2)
				ui.Row(c).Absolute().Top(0).Left(left).Height(t.Space(8)).
					AlignItems(ui.Center).Children(func() {
					// Badge pins itself to the cross-start; centre it on the
					// header's title line instead.
					components.Badge(c, locales.CommonSelectionCount(len(l.chosen), len(l.rows))).
						AlignSelf(ui.Center)
				})
			}
		})
	}
}

func (l *View) cell(c *ui.Context, row, col int) {
	pkg := l.rows[row]
	switch col {
	case 0:
		checked := l.isChosen(pkg)
		// 6 DIPs past the cell padding, to line up with the toolbar's inset.
		ui.Row(c).Padding(0, 0, 0, 6).Children(func() {
			box := components.Checkbox(c, &checked, "").Label(pkg.Name)
			if box.Changed() {
				l.choose(pkg, checked)
			}
		})
	case l.actions:
		l.rowActions(c, pkg)
	default:
		parts.Cell(c, pkg, col-1, l.Data.Name)
	}
}

func (l *View) rowActions(c *ui.Context, pkg manager.Package) {
	caps := l.Data.Capabilities(pkg.Manager)
	ui.Row(c).Gap(2).AlignItems(ui.Center).Justify(ui.End).Children(func() {
		running := l.Data.Running(pkg)
		updating := running == "update"
		canUpdate := pkg.Outdated && caps.Update && running == ""
		updateLabel := locales.ActionsUpdate()
		if updating {
			updateLabel = locales.ActionsUpdating()
		}
		updateSlot := parts.ActionSlot(c)
		updateSlot.Children(func() {
			update := components.IconButton(c, lucide.CircleArrowUp(), updateLabel, components.IconButtonPlain)
			update.Disabled(!canUpdate)
			if update.Clicked() && canUpdate && l.OnUpdate != nil {
				l.OnUpdate(pkg, flight.OriginOf(update))
			}
		})
		switch {
		case updating:
			// The label already says it is working.
		case running != "":
			parts.ActionTooltip(c, updateSlot, canUpdate, locales.ActionWorking())
		case !l.Data.Enabled(pkg.Manager):
			parts.ActionTooltip(c, updateSlot, canUpdate, locales.ActionManagerOff())
		case !caps.Update:
			parts.ActionTooltip(c, updateSlot, canUpdate, locales.ActionUnsupported())
		case !pkg.Outdated:
			parts.ActionTooltip(c, updateSlot, canUpdate, locales.ActionUpToDate())
		}

		if !l.AllowRemove {
			return
		}
		removing := running == "remove"
		canRemove := caps.Uninstall && running == ""
		removeLabel := locales.ActionsRemove()
		if removing {
			removeLabel = locales.ActionsRemoving()
		}
		removeSlot := parts.ActionSlot(c)
		removeSlot.Children(func() {
			remove := components.IconButton(c, lucide.Trash(), removeLabel, components.IconButtonPlain)
			remove.TextColor(c.Theme().Danger)
			remove.Disabled(!canRemove)
			if remove.Clicked() && canRemove {
				l.uninstallTarget = pkg
				l.uninstallOpen = true
			}
		})
		switch {
		case removing:
			// Its own label already says it is working.
		case running != "":
			parts.ActionTooltip(c, removeSlot, canRemove, locales.ActionWorking())
		case !l.Data.Enabled(pkg.Manager):
			parts.ActionTooltip(c, removeSlot, canRemove, locales.ActionManagerOff())
		case !caps.Uninstall:
			parts.ActionTooltip(c, removeSlot, canRemove, locales.ActionUnsupported())
		}
	})
}

// uninstallDialog asks before removing one package; the removal itself is a
// hold, so a stray click cannot confirm it.
func (l *View) uninstallDialog(c *ui.Context) {
	components.Dialog(c, &l.uninstallOpen, components.DialogOptions{Label: locales.UninstallTitle()}, func(panel ui.Element) {
		pkg := l.uninstallTarget
		t := c.Theme()
		panel.Width(420).Padding(20).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, locales.UninstallTitle()).FontSize(16).Bold()
				components.Text(c, locales.UninstallMessage(pkg.Name, l.Data.Name(pkg.Manager))).
					TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					cancel := components.Button(c, locales.CommonCancel(), components.ButtonOutline)
					if cancel.Clicked() {
						l.uninstallOpen = false
					}
					var confirmed bool
					hold := components.HoldToConfirm(c, &confirmed, locales.UninstallConfirm(),
						components.HoldToConfirmOptions{Tone: components.HoldDanger})
					if hold.Changed() {
						l.uninstallOpen = false
						if l.OnRemove != nil {
							l.OnRemove(pkg, flight.OriginOf(hold.Element))
						}
					}
				})
			})
		})
	})
}

func (l *View) bulkDialog(c *ui.Context) {
	components.Dialog(c, &l.bulkOpen, components.DialogOptions{Label: locales.UninstallBulkTitle()}, func(panel ui.Element) {
		targets := l.bulkTargets
		t := c.Theme()
		panel.Width(440).Padding(20).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, locales.UninstallBulkTitle()).FontSize(16).Bold()
				components.Text(c, locales.UninstallBulkMessage(len(targets))).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					cancel := components.Button(c, locales.CommonCancel(), components.ButtonOutline)
					if cancel.Clicked() {
						l.bulkOpen = false
					}
					var confirmed bool
					hold := components.HoldToConfirm(c, &confirmed, locales.UninstallConfirm(),
						components.HoldToConfirmOptions{Tone: components.HoldDanger})
					if hold.Changed() {
						l.bulkOpen = false
						if l.OnBulkRemove != nil {
							l.OnBulkRemove(targets, flight.OriginOf(hold.Element))
						}
					}
				})
			})
		})
	})
}
