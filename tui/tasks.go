package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MH4GF/tq/db"
)

type clearTasksMessageMsg struct {
	gen int
}

type projectTree struct {
	project db.Project
	tasks   []taskNode
}

type taskNode struct {
	task    db.Task
	actions []db.Action
}

type tasksMode int

const (
	modeNormal tasksMode = iota
	modeViewDetail
	modeViewTaskDetail
)

type lineType int

const (
	lineProject lineType = iota
	lineTask
	lineAction
	lineCardTop
	lineCardBottom
	lineCardSep
)

// decorative reports whether the line is a card border (non-selectable).
func (lt lineType) decorative() bool {
	return lt == lineCardTop || lt == lineCardBottom || lt == lineCardSep
}

type TasksModel struct {
	trees    []projectTree
	cursor   int
	expanded map[string]bool
	lines    []treeLine
	width    int
	height   int
	database db.Store
	// dispatchFn is optional: nil disables the 'd' keybinding and its help
	// entry. Set by New; NewTasksModel callers (tests) leave it nil.
	dispatchFn     DispatchFunc
	message        string
	messageGen     int
	messageIsError bool
	dateFilter     string

	// Cached stats
	runningInteractiveOrBg int
	depsByAction           map[int64][]db.ActionDepStatus

	// Detail view state
	mode          tasksMode
	detailAction  *db.Action
	detailTask    *db.Task
	detailNotes   []db.TaskNoteEntry
	detailHistory []db.TaskStatusHistoryEntry
	detailScroll  int
}

type treeLine struct {
	text       string
	key        string
	expandKey  string
	taskID     int64
	projectID  int64
	action     *db.Action
	rightLabel string
	lineType   lineType
}

type actionAttachedMsg struct {
	id      int64
	message string
}

type actionResumedMsg struct {
	parentID int64
	newID    int64
	err      error
}

type tasksLoadedMsg struct {
	trees                  []projectTree
	runningInteractiveOrBg int
	depsByAction           map[int64][]db.ActionDepStatus
	err                    error
}

type dispatchToggledMsg struct {
	err error
}

type actionDispatchedMsg struct {
	id     int64
	output string
	err    error
}

// actionStats holds aggregate counts for the status strip and gauge.
type actionStats struct {
	running                int
	runningInteractiveOrBg int
	pending                int
	done                   int
	failed                 int
	pendingLabel           string
}

func NewTasksModel(database db.Store, dateFilter string) TasksModel {
	return TasksModel{
		database:   database,
		expanded:   make(map[string]bool),
		dateFilter: dateFilter,
	}
}

func (m TasksModel) loadTasks() tea.Cmd {
	return func() tea.Msg {
		var firstErr error
		recordErr := func(err error) {
			if firstErr == nil {
				firstErr = err
			}
		}

		projects, err := m.database.ListProjects(0)
		if err != nil {
			return tasksLoadedMsg{err: fmt.Errorf("list projects: %w", err)}
		}
		sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })

		projectIDs := make([]int64, len(projects))
		for i, p := range projects {
			projectIDs[i] = p.ID
		}
		tasksByProject, err := m.database.ListTasksByProjectIDs(projectIDs)
		if err != nil {
			recordErr(fmt.Errorf("list tasks: %w", err))
			tasksByProject = map[int64][]db.Task{}
		}
		var allTaskIDs []int64
		for _, p := range projects {
			for _, t := range tasksByProject[p.ID] {
				allTaskIDs = append(allTaskIDs, t.ID)
			}
		}

		actionsByTask, err := m.database.ListActionsByTaskIDsForView(allTaskIDs, m.dateFilter)
		if err != nil {
			recordErr(fmt.Errorf("list actions: %w", err))
			actionsByTask = map[int64][]db.Action{}
		}

		var trees []projectTree
		for _, p := range projects {
			tasks := tasksByProject[p.ID]

			var nodes []taskNode
			for _, t := range tasks {
				actions := actionsByTask[t.ID]
				if m.dateFilter != "" && db.IsTerminalTaskStatus(t.Status) {
					if !t.MatchesDate(m.dateFilter) && len(actions) == 0 {
						continue
					}
				}
				// SQL returns id DESC per task; stable status sort keeps
				// equal-status actions newest-first.
				sort.SliceStable(actions, func(i, j int) bool {
					return actionStatusOrder(actions[i].Status) < actionStatusOrder(actions[j].Status)
				})
				nodes = append(nodes, taskNode{task: t, actions: actions})
			}
			sort.SliceStable(nodes, func(i, j int) bool {
				return taskStatusOrder(nodes[i].task.Status) < taskStatusOrder(nodes[j].task.Status)
			})
			trees = append(trees, projectTree{project: p, tasks: nodes})
		}
		var ri int
		n, err := m.database.CountRunningInteractive()
		if err != nil {
			recordErr(fmt.Errorf("count running interactive: %w", err))
		} else {
			ri = n
		}
		var actionIDs []int64
		for _, as := range actionsByTask {
			for _, a := range as {
				actionIDs = append(actionIDs, a.ID)
			}
		}
		depsByAction, err := m.database.ListActionDependenciesByActionIDs(actionIDs)
		if err != nil {
			recordErr(fmt.Errorf("list action dependencies: %w", err))
			depsByAction = map[int64][]db.ActionDepStatus{}
		}
		return tasksLoadedMsg{trees: trees, runningInteractiveOrBg: ri, depsByAction: depsByAction, err: firstErr}
	}
}

func (m TasksModel) toggleDispatch(projectID int64, enabled bool) tea.Cmd {
	return func() tea.Msg {
		return dispatchToggledMsg{err: m.database.SetDispatchEnabled(projectID, enabled)}
	}
}

func (m TasksModel) Init() tea.Cmd {
	return m.loadTasks()
}

// setMessage sets a transient status message and returns its TTL clear cmd.
// Always use this instead of writing to message/messageIsError/messageGen
// directly — bumping messageGen is required for clearTasksMessageMsg to
// match the right generation, and a missed bump silently breaks TTL clearing.
func (m *TasksModel) setMessage(msg string, isError bool) tea.Cmd {
	m.message = msg
	m.messageIsError = isError
	m.messageGen++
	return clearAfterTTL(clearTasksMessageMsg{gen: m.messageGen})
}

func (m TasksModel) Update(msg tea.Msg) (TasksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case actionAttachedMsg:
		if msg.message != "" {
			return m, m.setMessage(msg.message, false)
		}
		return m, nil
	case actionResumedMsg:
		reload := m.loadTasks()
		var text string
		if msg.err != nil {
			text = fmt.Sprintf("resume failed: %v", msg.err)
		} else {
			text = fmt.Sprintf("resume action #%d created from #%d", msg.newID, msg.parentID)
		}
		return m, tea.Batch(reload, m.setMessage(text, msg.err != nil))
	case actionDispatchedMsg:
		reload := m.loadTasks()
		var text string
		if msg.err != nil {
			text = fmt.Sprintf("dispatch failed: %v", msg.err)
		} else {
			text = msg.output
		}
		return m, tea.Batch(reload, m.setMessage(text, msg.err != nil))
	case clearTasksMessageMsg:
		if msg.gen == m.messageGen {
			m.message = ""
			m.messageIsError = false
		}
		return m, nil
	case dispatchToggledMsg:
		reload := m.loadTasks()
		if msg.err != nil {
			return m, tea.Batch(reload, m.setMessage(fmt.Sprintf("toggle focus failed: %v", msg.err), true))
		}
		return m, reload
	case tasksLoadedMsg:
		var clearCmd tea.Cmd
		if msg.err != nil {
			clearCmd = m.setMessage(fmt.Sprintf("load tasks failed: %v", msg.err), true)
		}
		m.trees = msg.trees
		m.runningInteractiveOrBg = msg.runningInteractiveOrBg
		m.depsByAction = msg.depsByAction
		for _, pt := range m.trees {
			projKey := fmt.Sprintf("p:%d", pt.project.ID)
			if _, ok := m.expanded[projKey]; !ok {
				m.expanded[projKey] = pt.project.DispatchEnabled
			}
			for _, tn := range pt.tasks {
				taskKey := fmt.Sprintf("t:%d", tn.task.ID)
				if _, ok := m.expanded[taskKey]; !ok {
					m.expanded[taskKey] = tn.task.Status != db.TaskStatusDone && tn.task.Status != db.TaskStatusArchived
				}
			}
		}
		m.buildLines()
		if m.cursor >= len(m.lines) {
			m.cursor = max(0, len(m.lines)-1)
		}
		// Ensure cursor is on a selectable line
		m.skipDecorativeLines(1)
		return m, clearCmd

	case tea.KeyMsg:
		switch m.mode {
		case modeViewDetail, modeViewTaskDetail:
			return m.updateViewDetail(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m TasksModel) updateNormal(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.cursor < len(m.lines)-1 {
			m.cursor++
			m.skipDecorativeLines(1)
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.cursor > 0 {
			m.cursor--
			m.skipDecorativeLines(-1)
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
		if m.cursor < len(m.lines) {
			ek := m.lines[m.cursor].expandKey
			if ek != "" {
				m.expanded[ek] = !m.expanded[ek]
				m.buildLines()
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("v"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			line := m.lines[m.cursor]
			if a := line.action; a != nil {
				m.detailAction = a
				m.detailScroll = 0
				m.mode = modeViewDetail
			} else if line.lineType == lineTask && line.taskID > 0 {
				return m, m.openTaskDetail(line.taskID)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			if a := m.lines[m.cursor].action; actionAttachable(a) {
				return m, m.attachAction(a)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			if a := m.lines[m.cursor].action; a != nil && actionResumable(a) {
				return m, m.resumeAction(a)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
		if m.dispatchFn != nil && m.cursor >= 0 && m.cursor < len(m.lines) {
			if a := m.lines[m.cursor].action; a != nil && a.Status == db.ActionStatusPending {
				progress := m.setMessage(fmt.Sprintf("dispatching #%d...", a.ID), false)
				return m, tea.Batch(progress, m.dispatchAction(a))
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("f"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			if pid := m.lines[m.cursor].projectID; pid > 0 && m.lines[m.cursor].taskID == 0 {
				for _, pt := range m.trees {
					if pt.project.ID == pid {
						return m, m.toggleDispatch(pid, !pt.project.DispatchEnabled)
					}
				}
			}
		}
	}
	return m, nil
}

// skipDecorativeLines moves cursor past card border lines. If the requested
// direction runs off the end, it falls back to the opposite direction so the
// cursor never rests on a decorative line when a selectable line exists.
func (m *TasksModel) skipDecorativeLines(dir int) {
	if len(m.lines) == 0 {
		m.cursor = 0
		return
	}
	for m.cursor >= 0 && m.cursor < len(m.lines) && m.lines[m.cursor].lineType.decorative() {
		m.cursor += dir
	}
	if m.cursor >= 0 && m.cursor < len(m.lines) {
		return
	}
	// Ran off the end; search back from the clamped edge for a selectable line.
	m.cursor = max(0, min(m.cursor, len(m.lines)-1))
	for m.cursor >= 0 && m.cursor < len(m.lines) && m.lines[m.cursor].lineType.decorative() {
		m.cursor -= dir
	}
	m.cursor = max(0, min(m.cursor, len(m.lines)-1))
}

func (m TasksModel) updateViewDetail(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc"))):
		m.detailAction = nil
		m.detailTask = nil
		m.detailNotes = nil
		m.detailHistory = nil
		m.detailScroll = 0
		m.mode = modeNormal
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		m.detailScroll++
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.detailScroll > 0 {
			m.detailScroll--
		}
	}
	return m, nil
}

func (m *TasksModel) openTaskDetail(taskID int64) tea.Cmd {
	m.detailTask = nil
	m.detailHistory = nil
	m.detailNotes = nil

	task, err := m.database.GetTask(taskID)
	if err != nil {
		slog.Warn("tui: load task detail failed", "task_id", taskID, "err", err)
		return m.setMessage(fmt.Sprintf("open task detail failed: %v", err), true)
	}
	m.detailTask = task

	var partial []string
	if hist, err := m.database.TaskStatusHistory(taskID); err == nil {
		m.detailHistory = hist
	} else {
		slog.Warn("tui: load task status history failed", "task_id", taskID, "err", err)
		partial = append(partial, "history")
	}
	if notes, err := m.database.TaskNotes(taskID, ""); err == nil {
		m.detailNotes = notes
	} else {
		slog.Warn("tui: load task notes failed", "task_id", taskID, "err", err)
		partial = append(partial, "notes")
	}

	m.detailScroll = 0
	m.mode = modeViewTaskDetail

	if len(partial) > 0 {
		return m.setMessage(fmt.Sprintf("task detail partial: %s unavailable", strings.Join(partial, ", ")), true)
	}
	return nil
}

func cardBorder(left, right string, width int) string {
	return styleBorderChar.Render(left) + styleBorderChar.Render(strings.Repeat("─", max(0, width-2))) + styleBorderChar.Render(right)
}

func (m *TasksModel) buildLines() {
	m.lines = nil
	for _, pt := range m.trees {
		projKey := fmt.Sprintf("p:%d", pt.project.ID)
		arrow := "▸"
		if m.expanded[projKey] {
			arrow = "▾"
		}

		// Card top border
		projLabel := styleProjectName.Render(pt.project.Name)
		if !pt.project.DispatchEnabled {
			projLabel = styleMuted.Render("⊘ " + pt.project.Name)
		}

		taskCount := len(pt.tasks)
		badge := ""
		if taskCount > 0 {
			badge = styleBadge.Render(fmt.Sprintf(" %d tasks", taskCount))
		}

		rightLabel := pt.project.WorkDir

		// Build the project header text
		headerText := fmt.Sprintf("%s %s%s", styleBorderChar.Render(arrow), projLabel, badge)

		// Top border line
		m.lines = append(m.lines, treeLine{
			text:     cardBorder("┌", "┐", m.width),
			lineType: lineCardTop,
		})

		// Project header (selectable)
		m.lines = append(m.lines, treeLine{
			text:       styleBorderChar.Render("│") + " " + headerText,
			key:        projKey,
			expandKey:  projKey,
			projectID:  pt.project.ID,
			rightLabel: rightLabel,
			lineType:   lineProject,
		})

		if !m.expanded[projKey] {
			// Bottom border
			m.lines = append(m.lines, treeLine{
				text:     cardBorder("└", "┘", m.width),
				lineType: lineCardBottom,
			})
			continue
		}

		// Separator between project header and first task
		if len(pt.tasks) > 0 {
			m.lines = append(m.lines, treeLine{
				text:     cardBorder("│", "│", m.width),
				lineType: lineCardSep,
			})
		}

		for ti, tn := range pt.tasks {
			// Blank line between tasks
			if ti > 0 {
				m.lines = append(m.lines, treeLine{
					text:     padWithBorder(styleBorderChar.Render("│"), m.width, styleBorderChar.Render("│")),
					lineType: lineCardSep,
				})
			}

			taskKey := fmt.Sprintf("t:%d", tn.task.ID)
			tArrow := "▸"
			if m.expanded[taskKey] {
				tArrow = "▾"
			}

			// Task status styling
			isTerminal := tn.task.Status == db.TaskStatusDone || tn.task.Status == db.TaskStatusArchived
			var taskText string
			if isTerminal {
				ds := StatusDimStyle(tn.task.Status)
				taskText = fmt.Sprintf(" %s %s %s %s",
					styleBorderChar.Render(tArrow),
					ds.Render(StatusIcon(tn.task.Status)),
					ds.Render(fmt.Sprintf("#%d", tn.task.ID)),
					ds.Render(tn.task.Title),
				)
			} else {
				taskText = fmt.Sprintf(" %s %s %s %s",
					styleBorderChar.Render(tArrow),
					stylePending.Render("○"),
					styleMuted.Render(fmt.Sprintf("#%d", tn.task.ID)),
					tn.task.Title,
				)
			}

			m.lines = append(m.lines, treeLine{
				text:      styleBorderChar.Render("│") + taskText,
				key:       taskKey,
				expandKey: taskKey,
				taskID:    tn.task.ID,
				lineType:  lineTask,
			})

			if !m.expanded[taskKey] {
				continue
			}

			for _, a := range tn.actions {
				icon := StatusIcon(a.Status)
				var actionText string
				idStr := styleMuted.Render(fmt.Sprintf("#%d", a.ID))
				if a.Status == db.ActionStatusDone || a.Status == db.ActionStatusFailed || a.Status == db.ActionStatusCancelled {
					ds := StatusDimStyle(a.Status)
					actionText = fmt.Sprintf("     %s %s %s", ds.Render(icon), idStr, ds.Render(a.Title))
				} else {
					actionText = fmt.Sprintf("     %s %s %s", StatusStyle(a.Status).Render(icon), idStr, a.Title)
					if a.Status == db.ActionStatusPending {
						if s := blockedSuffix(m.depsByAction[a.ID]); s != "" {
							actionText += " " + stylePending.Render(s)
						}
					}
				}

				m.lines = append(m.lines, treeLine{
					text:     styleBorderChar.Render("│") + actionText,
					key:      fmt.Sprintf("a:%d", a.ID),
					taskID:   tn.task.ID,
					action:   &a,
					lineType: lineAction,
				})
			}
		}

		// Bottom border
		m.lines = append(m.lines, treeLine{
			text:     cardBorder("└", "┘", m.width),
			lineType: lineCardBottom,
		})

	}
}

func (m TasksModel) visibleRange() visibleRange {
	return calcVisibleRange(m.cursor, len(m.lines), m.height, 0)
}

func (m TasksModel) View() string {
	if m.mode == modeViewDetail && m.detailAction != nil {
		return RenderDetailView(m.detailAction, m.depsByAction[m.detailAction.ID], m.detailScroll, m.width, m.height)
	}

	if m.mode == modeViewTaskDetail && m.detailTask != nil {
		return RenderTaskDetailView(m.detailTask, m.detailHistory, m.detailNotes, m.detailScroll, m.width, m.height)
	}

	if len(m.lines) == 0 {
		return styleMuted.Render("  No tasks found")
	}

	var b strings.Builder

	visible := m.visibleRange()
	for i := visible.start; i < visible.end; i++ {
		line := m.lines[i]

		// Decorative lines (borders) — render as-is
		if line.lineType.decorative() {
			b.WriteString(line.text + "\n")
			continue
		}

		rendered := line.text
		rightBorder := styleBorderChar.Render("│")
		borderW := lipgloss.Width(rightBorder)

		// Inline result for cursor on action
		if i == m.cursor && line.action != nil && line.action.Result.Valid && line.action.Result.String != "" {
			label := "result"
			rst := StatusStyle(line.action.Status)
			lineWidth := lipgloss.Width(rendered)
			remaining := m.width - lineWidth - borderW - len(label) - 4
			if remaining > 10 {
				rendered += "  " + rst.Render(label+": "+truncateResult(line.action.Result.String, remaining))
			}
		}

		// Right-aligned label (workdir for projects)
		if line.rightLabel != "" {
			lineWidth := lipgloss.Width(rendered)
			label := line.rightLabel
			// 1sp gap before right border
			availW := m.width - lineWidth - borderW - 2 // 1sp min gap left + 1sp right
			if availW < 10 {
				// Not enough room, skip label
				rendered = padWithBorder(rendered, m.width, rightBorder)
			} else {
				if len(label) > availW {
					label = "..." + label[len(label)-availW+3:]
				}
				labelRendered := styleWorkDir.Render(label)
				labelWidth := lipgloss.Width(labelRendered)
				pad := m.width - lineWidth - labelWidth - borderW - 1 // 1sp before border
				pad = max(pad, 1)
				rendered += strings.Repeat(" ", pad) + labelRendered + " " + rightBorder
			}
		} else {
			rendered = padWithBorder(rendered, m.width, rightBorder)
		}

		// Cursor highlight
		if i == m.cursor {
			rendered = highlightLine(rendered, m.width)
		}

		b.WriteString(rendered + "\n")
	}

	if m.message != "" {
		style := styleDone
		if m.messageIsError {
			style = styleWarning
		}
		b.WriteString("\n  " + style.Render(m.message) + "\n")
	}

	return b.String()
}

func actionResumable(a *db.Action) bool {
	if a == nil || !db.IsTerminalActionStatus(a.Status) {
		return false
	}
	return actionClaudeSessionID(a) != ""
}

func actionClaudeSessionID(a *db.Action) string {
	if a == nil || a.Metadata == "" || a.Metadata == "{}" {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(a.Metadata), &meta); err != nil {
		return ""
	}
	s, _ := meta["claude_session_id"].(string)
	return s
}

func (m TasksModel) resumeAction(a *db.Action) tea.Cmd {
	parentID := a.ID
	store := m.database
	return func() tea.Msg {
		newID, err := store.ResumeAction(parentID, db.ResumeOptions{})
		return actionResumedMsg{parentID: parentID, newID: newID, err: err}
	}
}

func (m TasksModel) dispatchAction(a *db.Action) tea.Cmd {
	id := a.ID
	fn := m.dispatchFn
	return func() tea.Msg {
		out, err := fn(context.Background(), id)
		return actionDispatchedMsg{id: id, output: out, err: err}
	}
}

func actionAttachable(a *db.Action) bool {
	if a == nil {
		return false
	}
	short := daemonShortFromMetadata(a.Metadata)
	return short != ""
}

func daemonShortFromMetadata(raw string) string {
	if raw == "" || raw == "{}" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	short, _ := m["daemon_short"].(string)
	return short
}

func (m TasksModel) attachAction(a *db.Action) tea.Cmd {
	return func() tea.Msg {
		if !actionAttachable(a) {
			return actionAttachedMsg{id: a.ID, message: "no daemon_short recorded yet"}
		}
		short := daemonShortFromMetadata(a.Metadata)
		return actionAttachedMsg{id: a.ID, message: fmt.Sprintf("run `claude attach %s` in a terminal to view this session", short)}
	}
}

// actionStats returns aggregate action counts across all trees.
func (m TasksModel) actionStats() actionStats {
	var stats actionStats
	var pc db.PendingCounts
	for _, pt := range m.trees {
		for _, tn := range pt.tasks {
			for _, a := range tn.actions {
				switch a.Status {
				case db.ActionStatusRunning:
					stats.running++
				case db.ActionStatusPending:
					pc.Total++
					if pt.project.DispatchEnabled {
						pc.Dispatchable++
					}
				case db.ActionStatusDone:
					stats.done++
				case db.ActionStatusFailed:
					stats.failed++
				}
			}
		}
	}
	stats.pending = pc.Total
	stats.pendingLabel = pc.Label()
	stats.runningInteractiveOrBg = m.runningInteractiveOrBg
	return stats
}

func taskStatusOrder(status string) int {
	switch status {
	case db.TaskStatusDone:
		return 1
	case db.TaskStatusOpen:
		return 2
	case db.TaskStatusArchived:
		return 3
	default:
		return 2
	}
}

func actionStatusOrder(status string) int {
	switch status {
	case db.ActionStatusDone:
		return 1
	case db.ActionStatusFailed, db.ActionStatusCancelled:
		return 2
	case db.ActionStatusRunning:
		return 3
	case db.ActionStatusDispatched:
		return 4
	case db.ActionStatusPending:
		return 5
	default:
		return 3
	}
}

func (m TasksModel) HelpKeys() []HelpKey {
	if m.mode == modeViewDetail || m.mode == modeViewTaskDetail {
		return detailHelpKeys()
	}
	keys := commonHelpKeys()
	if m.cursor >= 0 && m.cursor < len(m.lines) {
		line := m.lines[m.cursor]
		if line.expandKey != "" {
			keys = append(keys, HelpKey{"enter", "expand"})
		}
		if line.action != nil {
			if actionAttachable(line.action) {
				keys = append(keys, HelpKey{"o", "attach"})
			}
			keys = append(keys, HelpKey{"v", "view detail"})
			if actionResumable(line.action) {
				keys = append(keys, HelpKey{"r", "resume"})
			}
			if m.dispatchFn != nil && line.action.Status == db.ActionStatusPending {
				keys = append(keys, HelpKey{"d", "dispatch"})
			}
		}
		if line.lineType == lineTask && line.taskID > 0 {
			keys = append(keys, HelpKey{"v", "view task"})
		}
		if line.projectID > 0 && line.taskID == 0 {
			keys = append(keys, HelpKey{"f", "toggle focus"})
		}
	}
	return keys
}

func (m TasksModel) SetSize(w, h int) TasksModel {
	m.width = w
	m.height = h
	return m
}

func (m TasksModel) Mode() tasksMode {
	return m.mode
}
