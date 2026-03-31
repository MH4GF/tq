package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MH4GF/tq/db"
)

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
	trees      []projectTree
	cursor     int
	expanded   map[string]bool
	lines      []treeLine
	width      int
	height     int
	database   db.Store
	message    string
	dateFilter string

	// Detail view state
	mode         tasksMode
	detailAction *db.Action
	detailScroll int
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

type tasksLoadedMsg struct {
	trees []projectTree
}

// actionStats holds aggregate counts for the status strip and gauge.
type actionStats struct {
	running      int
	pending      int
	done         int
	failed       int
	pendingLabel string
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
		projects, err := m.database.ListProjects(0)
		if err != nil {
			return tasksLoadedMsg{}
		}
		sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })

		var trees []projectTree
		for _, p := range projects {
			tasks, err := m.database.ListTasksByProject(p.ID)
			if err != nil {
				continue
			}

			var nodes []taskNode
			for _, t := range tasks {
				taskID := t.ID
				actions, err := m.database.ListActions("", &taskID, 0)
				if err != nil {
					continue
				}
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
				nodes = append(nodes, taskNode{task: t, actions: actions})
			}
			sort.SliceStable(nodes, func(i, j int) bool {
				return taskStatusOrder(nodes[i].task.Status) < taskStatusOrder(nodes[j].task.Status)
			})
			trees = append(trees, projectTree{project: p, tasks: nodes})
		}
		return tasksLoadedMsg{trees: trees}
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
		}
		return m, nil
	case tasksLoadedMsg:
		m.trees = msg.trees
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
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeViewDetail:
			return m.updateViewDetail(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m TasksModel) updateNormal(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	m.message = ""
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
			if a := m.lines[m.cursor].action; a != nil && a.Result.Valid && a.Result.String != "" {
				m.detailAction = a
				m.detailScroll = 0
				m.mode = modeViewDetail
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			if a := m.lines[m.cursor].action; a != nil && a.SessionID.Valid {
				return m, m.attachAction(a)
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("f"))):
		if m.cursor >= 0 && m.cursor < len(m.lines) {
			if pid := m.lines[m.cursor].projectID; pid > 0 && m.lines[m.cursor].taskID == 0 {
				for _, pt := range m.trees {
					if pt.project.ID == pid {
						_ = m.database.SetDispatchEnabled(pid, !pt.project.DispatchEnabled)
						return m, m.loadTasks()
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

func cardBorder(left, right string, width int) string {
	return styleBorderChar.Render(left) + styleBorderChar.Render(strings.Repeat("─", max(0, width-2))) + styleBorderChar.Render(right)
}

func (m *TasksModel) buildLines() {
	m.lines = nil
	for pi, pt := range m.trees {
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
			if pi < len(m.trees)-1 {
				m.lines = append(m.lines, treeLine{text: "", lineType: lineCardSep})
			}
			continue
		}

		for ti, tn := range pt.tasks {
			taskKey := fmt.Sprintf("t:%d", tn.task.ID)
			tArrow := "▸"
			if m.expanded[taskKey] {
				tArrow = "▾"
			}

			// Task status styling
			isDone := tn.task.Status == db.TaskStatusDone || tn.task.Status == db.TaskStatusArchived
			var taskText string
			if isDone {
				taskText = fmt.Sprintf("  %s #%d %s",
					styleBorderChar.Render(tArrow),
					tn.task.ID,
					styleDoneDim.Render(tn.task.Title),
				)
			} else {
				actionCount := ""
				if len(tn.actions) > 0 {
					actionCount = styleBadge.Render(fmt.Sprintf(" %d actions", len(tn.actions)))
				}
				taskText = fmt.Sprintf("  %s %s %s%s",
					styleBorderChar.Render(tArrow),
					styleMuted.Render(fmt.Sprintf("#%d", tn.task.ID)),
					tn.task.Title,
					actionCount,
				)
			}

			// Separator between tasks
			if ti > 0 {
				m.lines = append(m.lines, treeLine{
					text:     cardBorder("│", "│", m.width),
					lineType: lineCardSep,
				})
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
				if a.Status == db.ActionStatusDone || a.Status == db.ActionStatusFailed {
					ds := StatusDimStyle(a.Status)
					actionText = fmt.Sprintf("    %s %s", ds.Render(icon), ds.Render(a.Title))
				} else {
					actionText = fmt.Sprintf("    %s %s", StatusStyle(a.Status).Render(icon), a.Title)
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

		// Gap between project cards
		if pi < len(m.trees)-1 {
			m.lines = append(m.lines, treeLine{text: "", lineType: lineCardSep})
		}
	}
}

func (m TasksModel) visibleRange() visibleRange {
	return calcVisibleRange(m.cursor, len(m.lines), m.height, 0)
}

func (m TasksModel) View() string {
	if m.mode == modeViewDetail && m.detailAction != nil {
		return RenderDetailView(m.detailAction, m.detailScroll, m.width, m.height)
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

		// Right-aligned label (workdir for projects)
		if line.rightLabel != "" {
			lineWidth := lipgloss.Width(rendered)
			labelRendered := styleWorkDir.Render(line.rightLabel)
			labelWidth := lipgloss.Width(labelRendered)
			borderRight := lipgloss.Width(styleBorderChar.Render("│"))
			pad := m.width - lineWidth - labelWidth - borderRight - 1
			if pad > 0 {
				rendered += strings.Repeat(" ", pad) + labelRendered + styleBorderChar.Render("│")
			}
		}

		// Cursor row: accent bar + highlight
		if i == m.cursor {
			// Inline result for action under cursor
			if line.action != nil && line.action.Result.Valid && line.action.Result.String != "" {
				label := "result"
				rst := StatusStyle(line.action.Status)
				lineWidth := lipgloss.Width(rendered)
				pad := 2
				labelLen := len(label) + 2
				remaining := m.width - lineWidth - pad - labelLen
				if remaining > 10 {
					rendered += "  " + rst.Render(label+": "+truncateResult(line.action.Result.String, remaining))
				}
			}

			rendered = highlightLine(rendered, m.width)
		}

		b.WriteString(rendered + "\n")
	}

	if m.message != "" {
		b.WriteString("\n  " + styleDone.Render(m.message) + "\n")
	}

	return b.String()
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

func (m TasksModel) HelpKeys() []HelpKey {
	if m.mode == modeViewDetail {
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
