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
}

type actionAttachedMsg struct {
	id      int64
	message string
}

type tasksLoadedMsg struct {
	trees []projectTree
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
		projects, err := m.database.ListProjects()
		if err != nil {
			return tasksLoadedMsg{}
		}

		var trees []projectTree
		for _, p := range projects {
			tasks, err := m.database.ListTasksByProject(p.ID)
			if err != nil {
				continue
			}

			var nodes []taskNode
			for _, t := range tasks {
				taskID := t.ID
				actions, err := m.database.ListActions("", &taskID)
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
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.cursor > 0 {
			m.cursor--
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

func (m *TasksModel) buildLines() {
	m.lines = nil
	for _, pt := range m.trees {
		projKey := fmt.Sprintf("p:%d", pt.project.ID)
		arrow := "▸"
		if m.expanded[projKey] {
			arrow = "▾"
		}
		projLabel := styleProject.Render(pt.project.Name)
		if !pt.project.DispatchEnabled {
			projLabel = styleMuted.Render("⊘ " + pt.project.Name)
		}
		rightLabel := pt.project.WorkDir
		m.lines = append(m.lines, treeLine{
			text:       fmt.Sprintf("%s %s", arrow, projLabel),
			key:        projKey,
			expandKey:  projKey,
			projectID:  pt.project.ID,
			rightLabel: rightLabel,
		})

		if !m.expanded[projKey] {
			continue
		}

		for _, tn := range pt.tasks {
			taskKey := fmt.Sprintf("t:%d", tn.task.ID)
			tArrow := "  ▸"
			if m.expanded[taskKey] {
				tArrow = "  ▾"
			}
			st := StatusStyle(tn.task.Status)
			m.lines = append(m.lines, treeLine{
				text:      fmt.Sprintf("%s #%d %s %s", tArrow, tn.task.ID, st.Render(tn.task.Status), tn.task.Title),
				key:       taskKey,
				expandKey: taskKey,
				taskID:    tn.task.ID,
			})

			if !m.expanded[taskKey] {
				continue
			}

			for _, a := range tn.actions {
				a := a
				icon := StatusIcon(a.Status)
				ast := StatusStyle(a.Status)
				m.lines = append(m.lines, treeLine{
					text: fmt.Sprintf("      %s %-4d %s %s",
						ast.Render(icon),
						a.ID,
						ast.Render(fmt.Sprintf("%-14s", a.Status)),
						a.Title,
					),
					key:    fmt.Sprintf("a:%d", a.ID),
					taskID: tn.task.ID,
					action: &a,
				})
			}
		}
	}
}

func (m TasksModel) visibleRange() visibleRange {
	return calcVisibleRange(m.cursor, len(m.lines), m.height, 3)
}

func (m TasksModel) View() string {
	if m.mode == modeViewDetail && m.detailAction != nil {
		return RenderDetailView(m.detailAction, m.detailScroll, m.width, m.height)
	}

	if len(m.lines) == 0 {
		return styleMuted.Render("  No tasks found")
	}

	var b strings.Builder
	b.WriteString(m.summaryLine() + "\n")

	visible := m.visibleRange()
	for i := visible.start; i < visible.end; i++ {
		line := m.lines[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		rendered := prefix + line.text

		if i == m.cursor && line.action != nil && line.action.Result.Valid && line.action.Result.String != "" {
			label := "result"
			rst := StatusStyle(line.action.Status)
			lineWidth := lipgloss.Width(rendered)
			pad := 2
			labelLen := len(label) + 2
			remaining := m.width - lineWidth - pad - labelLen
			if remaining > 10 {
				rendered += "  " + rst.Render(label+": "+truncateResult(line.action.Result.String, remaining))
			}
		} else if line.rightLabel != "" {
			lineWidth := lipgloss.Width(rendered)
			labelWidth := lipgloss.Width(line.rightLabel)
			pad := m.width - lineWidth - labelWidth - 2
			if pad > 0 {
				rendered += strings.Repeat(" ", pad) + styleMuted.Render(line.rightLabel)
			}
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

func (m TasksModel) summaryLine() string {
	var running, pending, done, failed int
	for _, pt := range m.trees {
		for _, tn := range pt.tasks {
			for _, a := range tn.actions {
				switch a.Status {
				case db.ActionStatusRunning:
					running++
				case db.ActionStatusPending:
					pending++
				case db.ActionStatusDone:
					done++
				case db.ActionStatusFailed:
					failed++
				}
			}
		}
	}
	return styleRunning.Render("●") + fmt.Sprintf(" %d running  ", running) +
		stylePending.Render("○") + fmt.Sprintf(" %d pending  ", pending) +
		styleDone.Render("✓") + fmt.Sprintf(" %d done  ", done) +
		styleFailed.Render("✗") + fmt.Sprintf(" %d failed", failed)
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
