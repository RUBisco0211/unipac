package views

import (
	"slices"
	"strconv"
	"strings"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
)

// listView renders a filterable, sortable, multi-selectable table of the shared
// inventory. The same type backs the installed and the outdated page.
type listView struct {
	data         *data
	outdatedOnly bool
	// allowRemove keeps uninstall off the outdated page, where the only
	// meaningful action is updating.
	allowRemove  bool
	onRefresh    func()
	onUpdate     func(manager.Package, actionOrigin)
	onRemove     func(manager.Package, actionOrigin)
	onBulkUpdate func([]manager.Package, actionOrigin)
	onBulkRemove func([]manager.Package, actionOrigin)

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
	chosen        map[string]struct{}
	selection     ui.Selection[any]
	sortOrder     ui.SortOrder

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

func (l *listView) view(c *ui.Context) {
	l.sync()
	ui.Column(c).Fill().Gap(12).Children(func() {
		ui.Column(c).Shrink(0).Gap(12).Padding(0, 16, 0, 16).Children(func() {
			l.toolbar(c)
			if l.data.refreshErr != nil {
				components.Text(c, locales.PackagesRefreshError(l.data.refreshErr.Error())).
					TextColor(c.Theme().Danger)
			}
		})
		// The slot is always reserved and the bar is drawn over it, so nothing
		// moves when a refresh starts or stops.
		ui.Column(c).Fill().Padding(2, 0, 0, 0).Children(func() {
			l.body(c)
			if l.data.refreshing {
				loadingBar(c)
			}
		})
	})
	l.uninstallDialog(c)
	l.bulkDialog(c)
}

// sync rebuilds the filtered rows when the inventory or a filter changed. It is
// pure in-memory work, safe inside a builder.
func (l *listView) sync() {
	l.syncManagerFilter()
	filter := l.filterKey()
	if l.applied.version == l.data.version &&
		l.applied.filter == filter &&
		l.applied.query == l.query {
		return
	}
	l.applied.version = l.data.version
	l.applied.filter = filter
	l.applied.query = l.query
	l.rebuild()
}

// syncManagerFilter keeps the multi-select valid when the manager list changes:
// a turned-off manager leaves the selection, and if everything was selected the
// new enabled set is selected too.
func (l *listView) syncManagerFilter() {
	if l.managers != nil && l.managersSeen == l.data.managersVersion {
		return
	}
	previousAll := l.managers == nil || l.allManagersSelectedFor(l.knownManagers)
	enabled := make([]string, 0, len(l.data.managers))
	next := make(map[string]bool, len(l.data.managers))
	known := make(map[string]bool, len(l.data.managers))
	for _, info := range l.data.managers {
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
	l.managersSeen = l.data.managersVersion
}

// allManagersSelectedFor reports whether the selection covers every manager in
// ids. It is used against the previous enabled set, before a newly enabled
// manager is added to the current one.
func (l *listView) allManagersSelectedFor(ids map[string]bool) bool {
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

func (l *listView) allManagersSelected() bool {
	enabled := 0
	for _, info := range l.data.managers {
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
func (l *listView) filterKey() string {
	ids := make([]string, 0, len(l.managers))
	for id, on := range l.managers {
		if on {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return strings.Join(ids, ",")
}

func (l *listView) rebuild() {
	l.rows = l.rows[:0]
	for _, pkg := range l.data.rows {
		if l.outdatedOnly && !pkg.Outdated {
			continue
		}
		if !l.managers[pkg.Manager] {
			continue
		}
		if !matchesQuery(pkg, l.query) {
			continue
		}
		l.rows = append(l.rows, pkg)
	}
	l.applySort()
	l.pruneSelection()
}

// pruneSelection drops keys that the current filters hide, so a bulk action
// never touches a package the user cannot see.
func (l *listView) pruneSelection() {
	visible := make(map[string]struct{}, len(l.rows))
	for _, pkg := range l.rows {
		visible[packageKey(pkg)] = struct{}{}
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
func (l *listView) isChosen(pkg manager.Package) bool {
	_, ok := l.chosen[packageKey(pkg)]
	return ok
}

// choose selects or deselects one package.
func (l *listView) choose(pkg manager.Package, on bool) {
	if l.chosen == nil {
		l.chosen = map[string]struct{}{}
	}
	key := packageKey(pkg)
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
func (l *listView) syncSelection() {
	l.selection.Clear()
	for key := range l.chosen {
		l.selection.Add(key)
	}
}

// selectColumnWidth is the width of the checkbox column.
const selectColumnWidth = 40

func (l *listView) columns() []ui.TableColumn {
	columns := []ui.TableColumn{{Title: "", ID: "select", Width: selectColumnWidth, Fixed: true}}
	columns = append(columns, packageFields()...)
	columns = append(columns, ui.TableColumn{
		Title: locales.TableActions(), ID: "actions", Width: 88, Fixed: true, Align: ui.End,
	})
	// A hair of trailing space so the last header and the row buttons do not
	// touch the window edge.
	return append(columns, ui.TableColumn{Title: "", ID: "pad", Width: 12, Fixed: true})
}

func (l *listView) applySort() {
	col := l.sortOrder.Column
	desc := l.sortOrder.Descending
	slices.SortStableFunc(l.rows, func(a, b manager.Package) int {
		var order int
		switch col {
		case "type":
			order = strings.Compare(typeLabel(a.IsGUI), typeLabel(b.IsGUI))
		case "manager":
			order = strings.Compare(
				strings.ToLower(l.data.managerName(a.Manager)),
				strings.ToLower(l.data.managerName(b.Manager)),
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

// matchesQuery reports whether a package matches the page's query, which is
// already lower-cased and trimmed.
func matchesQuery(pkg manager.Package, query string) bool {
	if query == "" {
		return true
	}
	query = strings.ToLower(strings.TrimSpace(query))
	return strings.Contains(strings.ToLower(pkg.Name), query) ||
		strings.Contains(strings.ToLower(pkg.Fullname), query) ||
		strings.Contains(strings.ToLower(pkg.Description), query)
}

func typeLabel(isGUI bool) string {
	if isGUI {
		return locales.TypeGui()
	}
	return locales.TypeCli()
}

func (l *listView) selectedPackages() []manager.Package {
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
func (l *listView) managerSelect(c *ui.Context) {
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
			for _, info := range l.data.managers {
				if info.Enabled {
					l.managers[info.ID] = all
				}
			}
		}
		for _, info := range l.data.managers {
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
func (l *listView) managerCounts() (map[string]int, int) {
	if l.counts != nil &&
		l.countsSeen.version == l.data.version &&
		l.countsSeen.managers == l.data.managersVersion &&
		l.countsSeen.query == l.query {
		return l.counts, l.countsTotal
	}
	counts := make(map[string]int, len(l.data.managers))
	total := 0
	for _, pkg := range l.data.rows {
		if l.outdatedOnly && !pkg.Outdated {
			continue
		}
		if !matchesQuery(pkg, l.query) || !l.data.enabled(pkg.Manager) {
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
	}{l.data.version, l.data.managersVersion, l.query}
	return counts, total
}

// managerLabel names the current filter in the trigger.
// allManagersOff reports whether every package belongs to a manager that is
// turned off, which is why none of them can be updated.
func (l *listView) allManagersOff(pkgs []manager.Package) bool {
	if len(pkgs) == 0 {
		return false
	}
	for _, pkg := range pkgs {
		if l.data.enabled(pkg.Manager) {
			return false
		}
	}
	return true
}

func (l *listView) managerLabel() string {
	enabled, selected := 0, 0
	only := ""
	for _, info := range l.data.managers {
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

func (l *listView) toolbar(c *ui.Context) {
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.End).Gap(8).Children(func() {
		components.TextBox(c, &l.query, components.TextBoxOptions{
			LeadingIcon: lucide.Search(),
		}).
			Placeholder(locales.PackagesSearchPlaceholder()).
			Label(locales.PackagesSearchPlaceholder()).
			Width(340)

		l.managerSelect(c)

		refreshLabel := locales.CommonRefresh()
		if l.data.refreshing {
			refreshLabel = locales.CommonRefreshing()
		}
		refresh := components.IconButton(c, lucide.RefreshCcw(), refreshLabel, components.IconButtonPlain)
		refresh.Disabled(l.data.refreshing)
		if refresh.Clicked() && l.onRefresh != nil {
			l.onRefresh()
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
			if pkg.Outdated && l.data.capabilities(pkg.Manager).Update &&
				l.data.actionFor(pkg) == "" {
				updatable = append(updatable, pkg)
			}
		}
		canBulkUpdate := len(updatable) > 0
		updateSlot := actionSlot(c)
		updateSlot.Children(func() {
			bulkUpdate := components.IconButton(c, lucide.CircleArrowUp(), locales.PackagesBulkUpdate(), components.IconButtonPlain)
			bulkUpdate.Disabled(!canBulkUpdate)
			if bulkUpdate.Clicked() && canBulkUpdate && l.onBulkUpdate != nil {
				l.onBulkUpdate(updatable, actionPoint(bulkUpdate))
			}
		})
		switch {
		case canBulkUpdate:
		case l.allManagersOff(selected):
			actionTooltip(c, updateSlot, canBulkUpdate, locales.ActionManagerOff())
		case l.data.anyActive(selected):
			actionTooltip(c, updateSlot, canBulkUpdate, locales.ActionWorking())
		default:
			actionTooltip(c, updateSlot, canBulkUpdate, locales.ActionUpToDate())
		}

		if !l.allowRemove {
			return
		}
		removable := make([]manager.Package, 0, len(selected))
		for _, pkg := range selected {
			if l.data.actionFor(pkg) == "" {
				removable = append(removable, pkg)
			}
		}
		canBulkRemove := len(removable) > 0
		removeSlot := actionSlot(c)
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
			actionTooltip(c, removeSlot, canBulkRemove, locales.ActionWorking())
		}
	})
}

// allSelected reports whether every visible row is chosen.
func (l *listView) allSelected() bool {
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
func (l *listView) toggleAll() {
	if l.chosen == nil {
		l.chosen = map[string]struct{}{}
	}
	if l.allSelected() {
		clear(l.chosen)
		l.syncSelection()
		return
	}
	for _, pkg := range l.rows {
		l.chosen[packageKey(pkg)] = struct{}{}
	}
	l.syncSelection()
}

func (l *listView) body(c *ui.Context) {
	switch {
	case l.data.loadErr != nil:
		placeholder(c, locales.PackagesLoadError(l.data.loadErr.Error()))
	case l.data.refreshing && len(l.rows) == 0:
		// Nothing cached yet: say what is happening instead of claiming the
		// inventory is empty.
		placeholder(c, locales.PackagesScanning())
	case len(l.rows) == 0:
		if l.outdatedOnly {
			placeholder(c, locales.OutdatedEmpty())
			return
		}
		placeholder(c, locales.PackagesEmpty())
	default:
		l.syncSelection()
		l.table.Sort = &l.sortOrder
		l.table.OnSort = func() { l.applySort() }
		l.table.List.Selection = &l.selection
		l.table.List.OnChange = l.syncSelection
		l.table.List.Key = func(row int) any { return packageKey(l.rows[row]) }
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

func (l *listView) cell(c *ui.Context, row, col int) {
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
	case 6:
		l.rowActions(c, pkg)
	default:
		packageFieldCell(c, pkg, col-1, l.data.managerName)
	}
}

func (l *listView) rowActions(c *ui.Context, pkg manager.Package) {
	caps := l.data.capabilities(pkg.Manager)
	ui.Row(c).Gap(2).AlignItems(ui.Center).Justify(ui.End).Children(func() {
		running := l.data.actionFor(pkg)
		updating := running == "update"
		canUpdate := pkg.Outdated && caps.Update && running == ""
		updateLabel := locales.ActionsUpdate()
		if updating {
			updateLabel = locales.ActionsUpdating()
		}
		updateSlot := actionSlot(c)
		updateSlot.Children(func() {
			update := components.IconButton(c, lucide.CircleArrowUp(), updateLabel, components.IconButtonPlain)
			update.Disabled(!canUpdate)
			if update.Clicked() && canUpdate && l.onUpdate != nil {
				l.onUpdate(pkg, actionPoint(update))
			}
		})
		switch {
		case updating:
			// The label already says it is working.
		case running != "":
			actionTooltip(c, updateSlot, canUpdate, locales.ActionWorking())
		case !l.data.enabled(pkg.Manager):
			actionTooltip(c, updateSlot, canUpdate, locales.ActionManagerOff())
		case !caps.Update:
			actionTooltip(c, updateSlot, canUpdate, locales.ActionUnsupported())
		case !pkg.Outdated:
			actionTooltip(c, updateSlot, canUpdate, locales.ActionUpToDate())
		}

		if !l.allowRemove {
			return
		}
		removing := running == "remove"
		canRemove := caps.Uninstall && running == ""
		removeLabel := locales.ActionsRemove()
		if removing {
			removeLabel = locales.ActionsRemoving()
		}
		removeSlot := actionSlot(c)
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
			actionTooltip(c, removeSlot, canRemove, locales.ActionWorking())
		case !l.data.enabled(pkg.Manager):
			actionTooltip(c, removeSlot, canRemove, locales.ActionManagerOff())
		case !caps.Uninstall:
			actionTooltip(c, removeSlot, canRemove, locales.ActionUnsupported())
		}
	})
}

// actionSlot wraps a control that may be disabled. MyGo shows no tooltip
// anchored on a disabled element, so the reason hangs on the wrapper instead.
func actionSlot(c *ui.Context) ui.Element {
	return ui.Row(c).Shrink(0).AlignItems(ui.Center)
}

// actionTooltip explains a disabled action; an enabled one needs no reason.
func actionTooltip(c *ui.Context, slot ui.Element, enabled bool, reason string) {
	if enabled || reason == "" {
		return
	}
	components.Tooltip(c, slot, reason)
}

// uninstallDialog asks before removing one package; the removal itself is a
// hold, so a stray click cannot confirm it.
func (l *listView) uninstallDialog(c *ui.Context) {
	components.Dialog(c, &l.uninstallOpen, components.DialogOptions{Label: locales.UninstallTitle()}, func(panel ui.Element) {
		pkg := l.uninstallTarget
		t := c.Theme()
		panel.Width(420).Padding(20).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				components.Text(c, locales.UninstallTitle()).FontSize(16).Bold()
				components.Text(c, locales.UninstallMessage(pkg.Name, l.data.managerName(pkg.Manager))).
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
						if l.onRemove != nil {
							l.onRemove(pkg, actionPoint(hold.Element))
						}
					}
				})
			})
		})
	})
}

func (l *listView) bulkDialog(c *ui.Context) {
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
						if l.onBulkRemove != nil {
							l.onBulkRemove(targets, actionPoint(hold.Element))
						}
					}
				})
			})
		})
	})
}
