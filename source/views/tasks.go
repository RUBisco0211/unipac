package views

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/RUBisco0211/unipac/source/backend/core/manager"
	"github.com/RUBisco0211/unipac/source/backend/tasks"
	"github.com/RUBisco0211/unipac/source/locales"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"
)

// terminalFont is the console's family: a monospaced face that shows column
// alignment the way the tool meant it.
const terminalFont = "SF Mono, Menlo, Consolas, monospace"

// tasksView is the tasks page: a filterable table of one operation per package,
// with the chosen task's console in a panel on the trailing edge. The panel is
// always there; the split divider resizes it.
type tasksView struct {
	// data supplies the action gate, so a retry waits like every other action.
	data *data

	all     []tasks.Snapshot
	rows    []tasks.Snapshot
	version int

	query string
	// opFilter and managerFilter are multi-selects: every option starts
	// selected. managerAll also selects managers that appear later, while an
	// explicit set leaves new managers unchecked.
	opFilter       map[tasks.Kind]bool
	operationsOpen bool
	managerFilter  map[string]bool
	managerAll     bool
	managersOpen   bool

	table         components.TableVirtualizedState
	selectedIndex int
	selectedID    string
	// splitPercent is the table pane's share of the page; the split writes it
	// as the user drags the divider.
	splitPercent float32

	// output is the selected task's console, loaded on demand from the task
	// manager. outputSeq is the snapshot sequence it corresponds to.
	output        tasks.Output
	outputFor     string
	outputSeq     int
	outputLoading bool

	onRetry  func(tasks.Snapshot, actionOrigin)
	onCancel func(tasks.Snapshot)
	onSelect func(string)

	applied struct {
		version int
		query   string
		filter  string
	}
	log    components.ListState
	logFor string
}

// apply installs a fresh set of snapshots. It runs on the UI thread, from the
// task listener or when the page is opened.
func (s *tasksView) apply(list []tasks.Snapshot) {
	s.all = list
	s.version++
	if s.selectedID != "" && !s.hasTask(s.selectedID) {
		s.clearSelection()
		return
	}
	if s.selectedID != "" && s.onSelect != nil {
		// Reload the console when new output arrived; loadTaskOutput compares
		// the sequence and leaves an unchanged console alone.
		s.onSelect(s.selectedID)
	}
}

func (s *tasksView) taskByID(id string) (tasks.Snapshot, bool) {
	for _, task := range s.all {
		if task.ID == id {
			return task, true
		}
	}
	return tasks.Snapshot{}, false
}

// reveal clears the filters and chooses id, for a notification click.
func (s *tasksView) reveal(id string) bool {
	if _, ok := s.taskByID(id); !ok {
		return false
	}
	s.query = ""
	s.opFilter = nil
	s.managerFilter = nil
	s.managerAll = true
	s.selectedID = id
	s.applied.version = -1
	return true
}

func (s *tasksView) hasTask(id string) bool {
	for _, task := range s.all {
		if task.ID == id {
			return true
		}
	}
	return false
}

func (s *tasksView) selected() (tasks.Snapshot, bool) {
	if s.selectedIndex < 0 || s.selectedIndex >= len(s.rows) {
		return tasks.Snapshot{}, false
	}
	return s.rows[s.selectedIndex], true
}

func (s *tasksView) busy(task tasks.Snapshot) bool {
	if s.data == nil {
		return false
	}
	return s.data.actionFor(manager.Package{Manager: task.ManagerID, Name: task.Package}) != ""
}

func (s *tasksView) runningCount() int {
	n := 0
	for _, task := range s.all {
		if task.State == tasks.StateRunning {
			n++
		}
	}
	return n
}

// loading reports whether the table is doing work: a task is queued or
// running, or a stored console is being read.
func (s *tasksView) loading() bool {
	if s.outputLoading {
		return true
	}
	for _, task := range s.all {
		if task.State == tasks.StateQueued || task.State == tasks.StateRunning {
			return true
		}
	}
	return false
}

func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}

// clearSelection drops the chosen task and any loaded console.
func (s *tasksView) clearSelection() {
	s.selectedID = ""
	s.selectedIndex = -1
	s.output = tasks.Output{}
	s.outputFor = ""
	s.outputSeq = 0
	s.outputLoading = false
}

// choose takes the row the table selected.
func (s *tasksView) choose() {
	if s.selectedIndex < 0 || s.selectedIndex >= len(s.rows) {
		s.clearSelection()
		return
	}
	s.selectedID = s.rows[s.selectedIndex].ID
	if s.onSelect != nil {
		s.onSelect(s.selectedID)
	}
}

// sync rebuilds the filtered rows when the snapshots or a filter changed. It is
// pure in-memory work, safe inside a builder.
var taskKinds = []tasks.Kind{tasks.KindInstall, tasks.KindUpdate, tasks.KindUninstall}

// ensureFilters starts both multi-selects with every option chosen. A test or a
// restored filter can provide its own maps first.
func (s *tasksView) ensureFilters() {
	if s.opFilter == nil {
		s.opFilter = make(map[tasks.Kind]bool, len(taskKinds))
		for _, kind := range taskKinds {
			s.opFilter[kind] = true
		}
	}
	if s.managerFilter == nil {
		s.managerFilter = make(map[string]bool)
		s.managerAll = true
	}
}

func (s *tasksView) allOperationsSelected() bool {
	for _, kind := range taskKinds {
		if !s.opFilter[kind] {
			return false
		}
	}
	return true
}

func (s *tasksView) allManagersSelected() bool {
	if s.managerAll {
		return true
	}
	options := s.managerOptions()
	if len(options) == 0 {
		return false
	}
	for _, option := range options {
		if !s.managerFilter[option.id] {
			return false
		}
	}
	return true
}

func (s *tasksView) operationLabel() string {
	if s.allOperationsSelected() {
		return locales.TasksAllOperations()
	}
	n, only := 0, ""
	for _, kind := range taskKinds {
		if s.opFilter[kind] {
			n++
			only = kindLabel(kind)
		}
	}
	if n == 1 {
		return only
	}
	return locales.OperationCount(n)
}

func (s *tasksView) managerFilterLabel() string {
	options := s.managerOptions()
	if s.managerAll || (len(options) > 0 && s.allManagersSelected()) {
		return locales.PackagesAllManagers()
	}
	n, only := 0, ""
	for _, option := range options {
		if s.managerFilter[option.id] {
			n++
			only = option.name
		}
	}
	if n == 1 {
		return only
	}
	return locales.ManagerCount(n)
}

// operationCounts counts the tasks of each kind after the manager filter and
// query, ignoring the operation filter, so an unchecked kind still shows its
// count.
func (s *tasksView) operationCounts() (map[tasks.Kind]int, int) {
	counts := make(map[tasks.Kind]int, len(taskKinds))
	total := 0
	for _, task := range s.all {
		if !s.managerAll && !s.managerFilter[task.ManagerID] {
			continue
		}
		if !s.matchesQuery(task) {
			continue
		}
		counts[task.Kind]++
		total++
	}
	return counts, total
}

// managerCounts counts the tasks of each manager after the operation filter and
// query, ignoring the manager filter.
func (s *tasksView) managerCounts() (map[string]int, int) {
	counts := make(map[string]int)
	total := 0
	for _, task := range s.all {
		if !s.opFilter[task.Kind] {
			continue
		}
		if !s.matchesQuery(task) {
			continue
		}
		counts[task.ManagerID]++
		total++
	}
	return counts, total
}

func (s *tasksView) filterKey() string {
	ops := make([]string, 0, len(taskKinds))
	for _, kind := range taskKinds {
		if s.opFilter[kind] {
			ops = append(ops, string(kind))
		}
	}
	managers := "*"
	if !s.managerAll {
		ids := make([]string, 0, len(s.managerFilter))
		for id, on := range s.managerFilter {
			if on {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		managers = strings.Join(ids, ",")
	}
	return strings.Join(ops, ",") + "|" + managers
}

func (s *tasksView) sync() {
	s.ensureFilters()
	filter := s.filterKey()
	if s.applied.version == s.version &&
		s.applied.query == s.query &&
		s.applied.filter == filter {
		return
	}
	s.applied = struct {
		version int
		query   string
		filter  string
	}{s.version, s.query, filter}

	s.rows = s.rows[:0]
	for _, task := range s.all {
		if !s.opFilter[task.Kind] {
			continue
		}
		if !s.managerAll && !s.managerFilter[task.ManagerID] {
			continue
		}
		if !s.matchesQuery(task) {
			continue
		}
		s.rows = append(s.rows, task)
	}

	if s.selectedID == "" {
		s.selectedIndex = -1
		return
	}
	for i := range s.rows {
		if s.rows[i].ID == s.selectedID {
			s.selectedIndex = i
			return
		}
	}
	s.clearSelection()
}

func (s *tasksView) matchesQuery(task tasks.Snapshot) bool {
	query := strings.ToLower(strings.TrimSpace(s.query))
	if query == "" {
		return true
	}
	haystack := strings.ToLower(
		task.Package + " " +
			task.ManagerName + " " +
			kindLabel(task.Kind) + " " +
			task.Command,
	)
	return strings.Contains(haystack, query)
}

type managerChoice struct{ id, name string }

func (s *tasksView) managerOptions() []managerChoice {
	// Only managers that actually have a task in this list: a manager with no
	// tasks would just be a zero row in the filter.
	names := make(map[string]string)
	for _, task := range s.all {
		names[task.ManagerID] = task.ManagerName
	}
	if s.data != nil {
		for _, info := range s.data.managers {
			if _, ok := names[info.ID]; ok {
				names[info.ID] = info.Name
			}
		}
	}
	options := make([]managerChoice, 0, len(names))
	for id, name := range names {
		options = append(options, managerChoice{id: id, name: name})
	}
	slices.SortFunc(options, func(a, b managerChoice) int {
		if order := strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name)); order != 0 {
			return order
		}
		return strings.Compare(a.id, b.id)
	})
	return options
}

func (s *tasksView) view(c *ui.Context) {
	s.sync()
	if s.splitPercent <= 0 {
		s.splitPercent = 64
	}
	// The shell's floating header overlays the whole window. The table starts
	// below it; the console's surface runs to the window's top edge, like the
	// sidebar's.
	top := sidebar.HeaderHeight(c) + 16
	split := components.SplitPercent(c, &s.splitPercent, func() {
		s.tablePane(c, top)
	}, func() {
		s.consolePane(c, top)
	}, components.SplitOptions{
		FirstMin:  360,
		SecondMin: 280,
		Default:   64,
	})
	split.Label(locales.TasksResizePanel())
	split.Fill()
}

// tablePane is the filter bar and the task table.
func (s *tasksView) tablePane(c *ui.Context, top float32) {
	ui.Column(c).Fill().MinWidth(0).Padding(top, 0, 0, 0).Children(func() {
		s.toolbar(c)
		ui.Box(c).Fill().MinHeight(0).Children(func() {
			s.body(c)
			if s.loading() {
				loadingBar(c)
			}
		})
	})
}

func (s *tasksView) toolbar(c *ui.Context) {
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(8).Wrap().
		Padding(2, 16, 10, 16).Children(func() {
		components.TextBox(c, &s.query, components.TextBoxOptions{
			LeadingIcon: lucide.Search(),
		}).
			Placeholder(locales.TasksSearchPlaceholder()).
			Label(locales.TasksSearchPlaceholder()).
			Width(260)

		s.operationSelect(c)
		s.managerSelect(c)
	})
}

func (s *tasksView) operationSelect(c *ui.Context) {
	t := c.Theme()
	trigger := components.Button(c, s.operationLabel(), components.ButtonOutline).
		Width(t.Space(37.5))
	trigger.Label(locales.TasksOperations())
	if trigger.Clicked() {
		s.operationsOpen = !s.operationsOpen
	}
	counts, total := s.operationCounts()
	components.Popover(c, trigger, &s.operationsOpen, func(panel ui.Element) {
		panel.Width(t.Space(50))
		components.Text(c, locales.TasksOperations()).FontSize(12).Bold()
		all := s.allOperationsSelected()
		allLabel := locales.TasksAllOperations() + " (" + strconv.Itoa(total) + ")"
		allBox := components.Checkbox(c, &all, allLabel)
		if allBox.Changed() {
			for _, kind := range taskKinds {
				s.opFilter[kind] = all
			}
		}
		for _, kind := range taskKinds {
			selected := s.opFilter[kind]
			label := kindLabel(kind) + " (" + strconv.Itoa(counts[kind]) + ")"
			box := components.Checkbox(c, &selected, label)
			if box.Changed() {
				s.opFilter[kind] = selected
			}
		}
	})
}

func (s *tasksView) managerSelect(c *ui.Context) {
	t := c.Theme()
	trigger := components.Button(c, s.managerFilterLabel(), components.ButtonOutline).
		Width(t.Space(40))
	trigger.Label(locales.SearchManagers())
	if trigger.Clicked() {
		s.managersOpen = !s.managersOpen
	}
	options := s.managerOptions()
	counts, total := s.managerCounts()
	components.Popover(c, trigger, &s.managersOpen, func(panel ui.Element) {
		panel.Width(t.Space(55))
		components.Text(c, locales.SearchManagers()).FontSize(12).Bold()
		all := s.managerAll || s.allManagersSelected()
		allLabel := locales.PackagesAllManagers() + " (" + strconv.Itoa(total) + ")"
		allBox := components.Checkbox(c, &all, allLabel)
		if allBox.Changed() {
			s.managerAll = all
			for _, option := range options {
				s.managerFilter[option.id] = all
			}
		}
		for _, option := range options {
			selected := s.managerAll || s.managerFilter[option.id]
			label := option.name + " (" + strconv.Itoa(counts[option.id]) + ")"
			box := components.Checkbox(c, &selected, label)
			if box.Changed() {
				if s.managerAll {
					s.managerAll = false
					for _, other := range options {
						s.managerFilter[other.id] = true
					}
				}
				s.managerFilter[option.id] = selected
			}
		}
	})
}

func (s *tasksView) body(c *ui.Context) {
	switch {
	case len(s.all) == 0:
		tasksPlaceholder(c, locales.TasksEmpty(), locales.TasksEmptyHint())
	case len(s.rows) == 0:
		tasksPlaceholder(c, locales.TasksNoMatch(), "")
	default:
		s.table.List.Selected = &s.selectedIndex
		s.table.List.Key = func(row int) any { return s.rows[row].ID }
		s.table.List.KeepOffset = true
		s.table.List.OnChange = s.choose
		// The table's own first-cell padding is a little tighter than the
		// toolbar; one spacing unit puts its first column under the search box.
		ui.Box(c).Fill().MinHeight(0).Padding(0, 0, 0, c.Theme().Space(1)).Children(func() {
			components.TableVirtualized(c, &s.table, s.columns(), len(s.rows), func(row, col int) {
				s.cell(c, row, col)
			}).Fill()
		})
	}
}

func tasksPlaceholder(c *ui.Context, text, hint string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(t.Space(2)).Padding(24).Children(func() {
		components.Icon(c, lucide.ListTodo).Size(t.Space(10), t.Space(10)).
			TextColor(t.TextMuted.Alpha(0.5))
		components.Text(c, text).TextColor(t.TextMuted)
		if hint != "" {
			components.Text(c, hint).FontSize(12).TextColor(t.TextMuted.Alpha(0.75))
		}
	})
}

func (s *tasksView) columns() []ui.TableColumn {
	return []ui.TableColumn{
		{Title: locales.TableOperation(), ID: "operation", Width: 100, Fixed: true},
		{Title: locales.TablePackage(), ID: "package", MinWidth: 120},
		{Title: locales.TableVersion(), ID: "version", Width: 124, Fixed: true},
		{Title: locales.TableManager(), ID: "manager", Width: 92, Fixed: true},
		{Title: locales.TableStatus(), ID: "status", Width: 92, Fixed: true},
		{Title: locales.TableStarted(), ID: "started", Width: 84, Fixed: true},
		{Title: locales.TableFinished(), ID: "finished", Width: 84, Fixed: true},
		{Title: "", ID: "pad", Width: 4, Fixed: true},
	}
}

func (s *tasksView) cell(c *ui.Context, row, col int) {
	task := s.rows[row]
	switch col {
	case 0:
		s.operationCell(c, task)
	case 1:
		// Two lines, like the package tables' name and description, so both
		// tables have the same row height.
		detail := firstLine(task.Command)
		if detail == "" {
			detail = taskMessage(task)
		}
		if detail == "" {
			detail = "-"
		}
		ui.Column(c).Gap(1).Children(func() {
			components.Tooltip(c, components.Text(c, task.Package).SingleLine(), task.Package)
			text := components.Text(c, detail).SingleLine().TextColor(c.Theme().TextMuted)
			if task.Command != "" {
				components.Tooltip(c, text, task.Command)
			}
		})
	case 2:
		versionCell(c, task)
	case 3:
		managerBadge(c, task.ManagerID, task.ManagerName)
	case 4:
		s.statusBadge(c, task).AlignSelf(ui.Center)
	case 5:
		clockCell(c, task.StartedAt)
	case 6:
		clockCell(c, task.FinishedAt)
	}
}

// clockLabel is compact enough for a column: today shows the clock, another
// day shows the date too. The tooltip has the full timestamp.
func clockLabel(at time.Time) string {
	if at.IsZero() {
		return "—"
	}
	now := time.Now()
	if at.Year() != now.Year() || at.YearDay() != now.YearDay() {
		return at.Format("01-02 15:04")
	}
	return at.Format("15:04:05")
}

func clockCell(c *ui.Context, at time.Time) {
	label := clockLabel(at)
	text := components.Text(c, label).SingleLine().TextColor(c.Theme().TextMuted)
	if at.IsZero() {
		return
	}
	components.Tooltip(c, text, at.Format("2006-01-02 15:04:05"))
}

func (s *tasksView) operationCell(c *ui.Context, task tasks.Snapshot) {
	icon, color := taskKind(task.Kind, c.Theme())
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		components.Icon(c, icon).Size(15, 15).Shrink(0).TextColor(color)
		components.Text(c, kindLabel(task.Kind)).SingleLine()
	})
}

func versionCell(c *ui.Context, task tasks.Snapshot) {
	t := c.Theme()
	old, new := task.Version, task.NewVersion
	switch task.Kind {
	case tasks.KindUpdate:
		if old != "" && new != "" && old != new {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				components.Text(c, old).SingleLine().TextColor(t.TextMuted)
				components.Text(c, "→").TextColor(t.TextMuted)
				components.Text(c, new).SingleLine().TextColor(t.Accent)
			})
			return
		}
		if new == "" {
			new = old
		}
		components.Text(c, orDash(new)).SingleLine()
	case tasks.KindInstall:
		if new == "" {
			new = old
		}
		components.Text(c, orDash(new)).SingleLine().TextColor(t.Accent)
	default:
		components.Text(c, orDash(old)).SingleLine().TextColor(t.TextMuted)
	}
}

func (s *tasksView) statusBadge(c *ui.Context, task tasks.Snapshot) ui.Element {
	if task.State == tasks.StateRunning {
		return runningBadge(c)
	}
	options := components.BadgeOptions{Tone: components.BadgeNeutral, Icon: lucide.Check()}
	switch task.State {
	case tasks.StateQueued:
		options.Tone = components.BadgeNeutral
		options.Icon = lucide.Clock()
	case tasks.StateFailed:
		options.Tone = components.BadgeDanger
		options.Icon = lucide.X()
	case tasks.StateCancelled:
		options.Tone = components.BadgeWarning
		options.Icon = lucide.Ban()
	default:
		options.Tone = components.BadgeSuccess
	}
	return components.Badge(c, statusLabel(task.State), options)
}

// runningBadge is the accent status pill with a turning Loader icon. It
// mirrors components.Badge's running style but builds the icon itself, because
// the badge's icon option is static and this one has to keep moving.
func runningBadge(c *ui.Context) ui.Element {
	t := c.Theme()
	size := t.Rem(.85)
	bg := t.Background.Mix(t.Accent, .12)
	fg := t.Accent.Mix(t.Text, .3)
	border := t.Background.Mix(t.Accent, .3)
	pill := ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).AlignSelf(ui.Start).
		Shrink(0).Clip().FontSize(size).FontWeight(500).
		Padding(t.Space(.5), t.Space(2)).Radius(t.Space(3)).
		Background(bg).TextColor(fg).Border(1, border)
	pill.Children(func() {
		icon := components.Icon(c, lucide.Loader()).Size(size, size).Shrink(0)
		if !c.Preferences().ReduceMotion {
			icon.Rotate(icon.Loop("tasks-running", 1200*time.Millisecond, ui.Linear) * 360)
		}
		components.Text(c, statusLabel(tasks.StateRunning)).SingleLine().Unselectable().Shrink(0)
	})
	return pill
}

// consolePane is the always-visible console panel.
func (s *tasksView) consolePane(c *ui.Context, top float32) {
	t := c.Theme()
	ui.Column(c).FillHeight().Background(t.Text.Alpha(0.03)).Children(func() {
		task, ok := s.selected()
		if !ok {
			ui.Column(c).Shrink(0).Padding(top+2, t.Space(4), t.Space(2), t.Space(4)).Children(func() {
				components.Text(c, locales.TasksConsole()).FontSize(13).Bold()
			})
			ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.Border)
			ui.Column(c).Fill().Center().Padding(20).Children(func() {
				components.Text(c, locales.TasksSelectTask()).FontSize(12).TextColor(t.TextMuted).Wrap()
			})
			return
		}

		ui.Column(c).Shrink(0).Gap(t.Space(1)).
			Padding(top+2, t.Space(4), t.Space(2), t.Space(4)).Children(func() {
			ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
					components.Text(c, taskTitle(task)).FontSize(13).Bold().SingleLine()
					components.Text(c, taskSubtitle(task)).FontSize(11.5).
						SingleLine().TextColor(t.TextMuted)
				})
				s.panelActions(c, task)
			})
			ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
				s.statusBadge(c, task).AlignSelf(ui.Center)
				if message := taskMessage(task); message != "" {
					text := components.Text(c, message).FontSize(11.5).
						SingleLine().MinWidth(0).Grow(1)
					if task.State == tasks.StateFailed {
						text.TextColor(t.Danger)
					} else {
						text.TextColor(t.TextMuted)
					}
					components.Tooltip(c, text, message)
				}
			})
		})

		ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.Border)

		if task.Command != "" {
			ui.Column(c).Shrink(0).Gap(t.Space(1)).
				Padding(t.Space(2), t.Space(4), t.Space(2), t.Space(4)).Children(func() {
				for _, command := range strings.Split(task.Command, "\n") {
					components.Text(c, "$ "+command).Font(terminalFont).FontSize(11.5).
						TextColor(t.TextMuted).Wrap()
				}
			})
		}
		if task.ExecPath != "" {
			ui.Row(c).Shrink(0).Gap(t.Space(1)).AlignItems(ui.Center).
				Padding(0, t.Space(4), t.Space(2), t.Space(4)).Children(func() {
				components.Icon(c, lucide.Terminal()).Size(13, 13).Shrink(0).TextColor(t.TextMuted)
				path := components.Text(c, task.ExecPath).Font(terminalFont).FontSize(11).
					SingleLine().TextColor(t.TextMuted)
				components.Tooltip(c, path, task.ExecPath)
			})
		}

		ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.Border.Alpha(0.6))
		ui.Box(c).Fill().MinHeight(0).Children(func() {
			s.consoleBody(c, task)
		})
	})
}

func (s *tasksView) panelActions(c *ui.Context, task tasks.Snapshot) {
	ui.Row(c).Gap(2).Shrink(0).AlignItems(ui.Center).Children(func() {
		switch task.State {
		case tasks.StateQueued, tasks.StateRunning:
			cancel := components.IconButton(c, lucide.CircleStop(), locales.TasksCancel(), components.IconButtonPlain)
			cancel.TextColor(c.Theme().Danger)
			if cancel.Clicked() && s.onCancel != nil {
				s.onCancel(task)
			}
		case tasks.StateFailed, tasks.StateCancelled:
			busy := s.busy(task)
			slot := actionSlot(c)
			slot.Children(func() {
				retry := components.Button(c, locales.TasksRetry(), components.ButtonOutline)
				retry.Disabled(busy)
				if retry.Clicked() && !busy && s.onRetry != nil {
					s.onRetry(task, actionPoint(retry))
				}
			})
			if busy {
				actionTooltip(c, slot, false, locales.ActionWorking())
			}
		}
	})
}

func (s *tasksView) consoleBody(c *ui.Context, task tasks.Snapshot) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(t.Space(2), t.Space(4), t.Space(2.5), t.Space(4)).Children(func() {
		if task.State == tasks.StateQueued {
			consoleNote(c, locales.TasksQueuedHint())
			return
		}

		output := s.taskOutput(task)
		if s.outputFor != task.ID || s.outputLoading {
			consoleNote(c, locales.TasksOutputLoading())
			return
		}
		if len(output.Lines) == 0 {
			consoleNote(c, locales.TasksNoOutput())
			return
		}

		if s.logFor != task.ID {
			s.logFor = task.ID
			s.log.ScrollToEnd()
		}
		if output.Truncated {
			components.Text(c, locales.TasksOutputTruncated()).FontSize(11).
				TextColor(t.TextMuted).Padding(0, 0, t.Space(1), t.Space(1))
		}
		s.log.FollowEnd = true
		components.List(c, &s.log, len(output.Lines), func(row int) {
			line := output.Lines[row]
			ui.Box(c).FillWidth().Padding(t.Space(0.5), 0).Children(func() {
				ui.RichText(c, ansiSpans(line.Text, t)...).
					Font(terminalFont).FontSize(11.5).
					TextColor(t.Text.Alpha(0.9)).Selectable()
			})
		}).Fill()
	})
}

func (s *tasksView) taskOutput(task tasks.Snapshot) tasks.Output {
	if s.outputFor == task.ID {
		return s.output
	}
	return tasks.Output{}
}

func consoleNote(c *ui.Context, text string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Padding(12).Children(func() {
		components.Text(c, text).FontSize(12).TextColor(t.TextMuted).Wrap()
	})
}

func taskTitle(task tasks.Snapshot) string {
	return kindLabel(task.Kind) + " · " + task.Package
}

func taskSubtitle(task tasks.Snapshot) string {
	parts := make([]string, 0, 3)
	if task.ManagerName != "" {
		parts = append(parts, task.ManagerName)
	}
	parts = append(parts, clockLabel(task.StartedAt))
	if !task.FinishedAt.IsZero() {
		parts = append(parts, "→", clockLabel(task.FinishedAt))
	}
	return strings.Join(parts, " ")
}

// taskMessage shows the one message UniPac writes itself in the window's
// language.
func taskMessage(task tasks.Snapshot) string {
	switch task.Message {
	case tasks.InterruptedMessage:
		return locales.TasksInterrupted()
	case tasks.CancelledMessage:
		return locales.TasksCancelledByUser()
	default:
		return task.Message
	}
}

func kindLabel(kind tasks.Kind) string {
	switch kind {
	case tasks.KindInstall:
		return locales.TasksKindInstall()
	case tasks.KindUpdate:
		return locales.TasksKindUpdate()
	case tasks.KindUninstall:
		return locales.TasksKindUninstall()
	default:
		return string(kind)
	}
}

func statusLabel(state tasks.State) string {
	switch state {
	case tasks.StateQueued:
		return locales.TasksStatusQueued()
	case tasks.StateRunning:
		return locales.TasksStatusRunning()
	case tasks.StateFailed:
		return locales.TasksStatusFailed()
	case tasks.StateCancelled:
		return locales.TasksStatusCancelled()
	default:
		return locales.TasksStatusCompleted()
	}
}

func taskKind(kind tasks.Kind, t *ui.Theme) (*ui.SVG, ui.Color) {
	switch kind {
	case tasks.KindInstall:
		return lucide.Download(), t.Success
	case tasks.KindUninstall:
		return lucide.Trash(), t.Danger
	default:
		return lucide.CircleArrowUp(), t.Accent
	}
}
