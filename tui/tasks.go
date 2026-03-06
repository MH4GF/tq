package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
}

type treeLine struct {
	text      string
	key       string
	expandKey string
	taskID    int64
}

type tasksLoadedMsg struct {
	trees []projectTree
}

// taskSelectedMsg is emitted when a user selects a task to view its actions.
type taskSelectedMsg struct {
	taskID int64
	title  string
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
		}
		m.buildLines()
		if m.cursor >= len(m.lines) {
			m.cursor = max(0, len(m.lines)-1)
		}
		return m, nil

	case tea.KeyMsg:
		return m.updateNormal(msg)
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
			line := m.lines[m.cursor]
			if line.expandKey != "" && line.taskID == 0 {
				// Project line: toggle expand/collapse
				m.expanded[line.expandKey] = !m.expanded[line.expandKey]
				m.buildLines()
			} else if line.taskID != 0 {
				// Task line: navigate to queue for this task
				var title string
				for _, pt := range m.trees {
					for _, tn := range pt.tasks {
						if tn.task.ID == line.taskID {
							title = tn.task.Title
						}
					}
				}
				return m, func() tea.Msg {
					return taskSelectedMsg{taskID: line.taskID, title: title}
				}
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		return m, m.loadTasks()
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
			st := StatusStyle(tn.task.Status)
			actionCount := len(tn.actions)
			m.lines = append(m.lines, treeLine{
				text:   fmt.Sprintf("  #%d %s %s (%d actions)", tn.task.ID, st.Render(tn.task.Status), tn.task.Title, actionCount),
				key:    fmt.Sprintf("t:%d", tn.task.ID),
				taskID: tn.task.ID,
			})
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
