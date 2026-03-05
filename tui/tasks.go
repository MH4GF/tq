package tui

import (
	"fmt"
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
	database   *db.DB
	message    string
	dateFilter string

	// Detail view state
	mode         tasksMode
	detailAction *db.Action
	detailScroll int
}

type treeLine struct {
	text      string
	key       string
	expandKey string
	taskID    int64
	action    *db.Action
}

type tasksLoadedMsg struct {
	trees []projectTree
}

func NewTasksModel(database *db.DB, dateFilter string) TasksModel {
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
					if t.Status == "done" || t.Status == "archived" {
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
			if len(nodes) > 0 {
				trees = append(trees, projectTree{project: p, tasks: nodes})
			}
		}
		return tasksLoadedMsg{trees: trees}
	}
}

func (m TasksModel) Init() tea.Cmd {
	return m.loadTasks()
}

func (m TasksModel) Update(msg tea.Msg) (TasksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tasksLoadedMsg:
		m.trees = msg.trees
		for _, pt := range m.trees {
			m.expanded[fmt.Sprintf("p:%d", pt.project.ID)] = true
			for _, tn := range pt.tasks {
				m.expanded[fmt.Sprintf("t:%d", tn.task.ID)] = tn.task.Status != "done" && tn.task.Status != "archived"
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
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		return m, m.loadTasks()
	}
	return m, nil
}

func (m TasksModel) updateViewDetail(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q"))):
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
		m.lines = append(m.lines, treeLine{
			text:      fmt.Sprintf("%s %s", arrow, styleProject.Render(pt.project.Name)),
			key:       projKey,
			expandKey: projKey,
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
					text: fmt.Sprintf("      %s %s %s",
						ast.Render(icon),
						ast.Render(fmt.Sprintf("%-14s", a.Status)),
						a.TemplateID,
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
	maxVisible := m.height - 2
	if maxVisible <= 0 {
		maxVisible = 20
	}
	total := len(m.lines)
	if total <= maxVisible {
		return visibleRange{0, total}
	}
	start := m.cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	return visibleRange{start, end}
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
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		rendered := prefix + line.text

		if i == m.cursor && line.action != nil && line.action.Result.Valid && line.action.Result.String != "" {
			label := "result"
			if line.action.Status == "waiting_human" {
				label = "reason"
			}
			rst := StatusStyle(line.action.Status)
			lineWidth := lipgloss.Width(rendered)
			pad := 2
			labelLen := len(label) + 2
			remaining := m.width - lineWidth - pad - labelLen
			if remaining > 10 {
				rendered += "  " + rst.Render(label+": "+truncateResult(line.action.Result.String, remaining))
			}
		}

		b.WriteString(rendered + "\n")
	}

	if m.message != "" {
		b.WriteString("\n  " + styleDone.Render(m.message) + "\n")
	}

	return b.String()
}

func (m TasksModel) SetSize(w, h int) TasksModel {
	m.width = w
	m.height = h
	return m
}

func (m TasksModel) Mode() tasksMode {
	return m.mode
}
