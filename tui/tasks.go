package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
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

type TasksModel struct {
	trees          []projectTree
	cursor         int
	expanded       map[string]bool
	lines          []treeLine
	width          int
	height         int
	database       db.Store
	message        string
	messageGen     int
	messageIsError bool
	dateFilter     string

	// Cached stats
	runningInteractive int

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
	trees              []projectTree
	runningInteractive int
	err                error
}

type dispatchToggledMsg struct {
	err error
}

// actionStats holds aggregate counts for the status strip and gauge.
type actionStats struct {
	running            int
	runningInteractive int
	pending            int
	done               int
	failed             int
	pendingLabel       string
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

		tasksByProject := make(map[int64][]db.Task, len(projects))
		var allTaskIDs []int64
		for _, p := range projects {
			tasks, err := m.database.ListTasksByProject(p.ID)
			if err != nil {
				recordErr(fmt.Errorf("list tasks for project %d: %w", p.ID, err))
				continue
			}
			tasksByProject[p.ID] = tasks
			for _, t := range tasks {
				allTaskIDs = append(allTaskIDs, t.ID)
			}
		}

		actionsByTask, err := m.database.ListActionsByTaskIDs(allTaskIDs)
		if err != nil {
			recordErr(fmt.Errorf("list actions: %w", err))
			actionsByTask = map[int64][]db.Action{}
		}

		var trees []projectTree
		for _, p := range projects {
			tasks, ok := tasksByProject[p.ID]
			if !ok {
				continue
			}

			var nodes []taskNode
			for _, t := range tasks {
				actions := actionsByTask[t.ID]
				// ListActionsByTaskIDs returns id ASC; legacy ListActions
				// used id DESC. Match the old order so the stable status
				// sort below leaves equal-status actions newest-first.
				sort.Slice(actions, func(i, j int) bool { return actions[i].ID > actions[j].ID })
				if m.dateFilter != "" {
					if t.Status == db.TaskStatusDone || t.Status == db.TaskStatusArchived {
						actions = db.FilterByDate(actions, m.dateFilter)
						if len(actions) == 0 && !t.MatchesDate(m.dateFilter) {
							continue
						}
					} else {
						actions = db.FilterForOpenTask(actions, m.dateFilter)
					}
				}
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
		return tasksLoadedMsg{trees: trees, runningInteractive: ri, err: firstErr}
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

func (m TasksModel) Update(msg tea.Msg) (TasksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case actionAttachedMsg:
		if msg.message != "" {
			m.message = msg.message
			m.messageIsError = false
			m.messageGen++
			return m, clearAfterTTL(clearTasksMessageMsg{gen: m.messageGen})
		}
		return m, nil
	case actionResumedMsg:
		reload := m.loadTasks()
		if msg.err != nil {
			m.message = fmt.Sprintf("resume failed: %v", msg.err)
			m.messageIsError = true
		} else {
			m.message = fmt.Sprintf("resume action #%d created from #%d", msg.newID, msg.parentID)
			m.messageIsError = false
		}
		m.messageGen++
		return m, tea.Batch(reload, clearAfterTTL(clearTasksMessageMsg{gen: m.messageGen}))
	case clearTasksMessageMsg:
		if msg.gen == m.messageGen {
			m.message = ""
			m.messageIsError = false
		}
		return m, nil
	case dispatchToggledMsg:
		reload := m.loadTasks()
		if msg.err != nil {
			m.message = fmt.Sprintf("toggle focus failed: %v", msg.err)
			m.messageIsError = true
			m.messageGen++
			return m, tea.Batch(reload, clearAfterTTL(clearTasksMessageMsg{gen: m.messageGen}))
		}
		return m, reload
	case tasksLoadedMsg:
		var clearCmd tea.Cmd
		if msg.err != nil {
			m.message = fmt.Sprintf("load tasks failed: %v", msg.err)
			m.messageIsError = true
			m.messageGen++
			clearCmd = clearAfterTTL(clearTasksMessageMsg{gen: m.messageGen})
		}
		m.trees = msg.trees
		m.runningInteractive = msg.runningInteractive
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
			if a := line.action; a != nil && a.Result.Valid && a.Result.String != "" {
				m.detailAction = a
				m.detailScroll = 0
				m.mode = modeViewDetail
			} else if line.lineType == lineTask && line.taskID > 0 {
				m.openTaskDetail(line.taskID)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			if a := m.lines[m.cursor].action; a != nil && a.SessionID.Valid {
				return m, m.attachAction(a)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			if a := m.lines[m.cursor].action; a != nil && actionResumable(a) {
				return m, m.resumeAction(a)
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

// skipDecorativeLines moves cursor past card border lines.
func (m *TasksModel) skipDecorativeLines(dir int) {
	for m.cursor >= 0 && m.cursor < len(m.lines) {
		lt := m.lines[m.cursor].lineType
		if lt != lineCardTop && lt != lineCardBottom && lt != lineCardSep {
			break
		}
		m.cursor += dir
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

func (m *TasksModel) openTaskDetail(taskID int64) {
	m.detailTask = nil
	m.detailHistory = nil
	m.detailNotes = nil
	task, err := m.database.GetTask(taskID)
	if err != nil {
		return
	}
	m.detailTask = task
	if hist, err := m.database.TaskStatusHistory(taskID); err == nil {
		m.detailHistory = hist
	}
	if notes, err := m.database.TaskNotes(taskID, ""); err == nil {
		m.detailNotes = notes
	}
	m.detailScroll = 0
	m.mode = modeViewTaskDetail
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
		return RenderDetailView(m.detailAction, m.detailScroll, m.width, m.height)
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
		if line.lineType == lineCardTop || line.lineType == lineCardBottom || line.lineType == lineCardSep {
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
	return actionSessionID(a) != ""
}

func actionSessionID(a *db.Action) string {
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

func (m TasksModel) attachAction(a *db.Action) tea.Cmd {
	return func() tea.Msg {
		target := fmt.Sprintf("%s:%s", a.SessionID.String, a.TmuxPane.String)
		if err := exec.Command("tmux", "select-window", "-t", target).Run(); err != nil {
			return actionAttachedMsg{id: a.ID, message: fmt.Sprintf("attach failed: %v", err)}
		}
		return actionAttachedMsg{id: a.ID}
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
	stats.runningInteractive = m.runningInteractive
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
			if line.action.SessionID.Valid {
				keys = append(keys, HelpKey{"o", "attach"})
			}
			if line.action.Result.Valid && line.action.Result.String != "" {
				keys = append(keys, HelpKey{"v", "view result"})
			}
			if actionResumable(line.action) {
				keys = append(keys, HelpKey{"r", "resume"})
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
