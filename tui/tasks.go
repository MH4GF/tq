package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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
	modePickTemplate
	modeInputInstruction
	modePickProject
	modeInputTitle
	modeInputURL
	modePickStatus
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
	tqDir      string
	message    string
	dateFilter string

	// Add action state
	mode           tasksMode
	targetTaskID   int64
	templates      []string
	templateCursor int
	textInput      textinput.Model

	// Create task state
	projects      []db.Project
	projectCursor int
	newTaskTitle   string

	// Status change state
	statuses     []string
	statusCursor int

	// Detail view state
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

type actionCreatedMsg struct {
	id       int64
	template string
	taskID   int64
}

type taskCreatedMsg struct {
	id      int64
	project string
	title   string
}

type taskUpdatedMsg struct {
	id     int64
	status string
}

func NewTasksModel(database *db.DB, dateFilter string) TasksModel {
	ti := textinput.New()
	ti.Placeholder = "instruction を入力..."
	ti.CharLimit = 500
	ti.Width = 60

	return TasksModel{
		database:   database,
		expanded:   make(map[string]bool),
		textInput:  ti,
		dateFilter: dateFilter,
	}
}

func (m *TasksModel) SetTQDir(dir string) {
	m.tqDir = dir
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

	case actionCreatedMsg:
		m.message = fmt.Sprintf("action #%d created (%s → task #%d)", msg.id, msg.template, msg.taskID)
		m.mode = modeNormal
		return m, m.loadTasks()

	case taskCreatedMsg:
		m.message = fmt.Sprintf("task #%d created (%s: %s)", msg.id, msg.project, msg.title)
		m.mode = modeNormal
		return m, m.loadTasks()

	case taskUpdatedMsg:
		m.message = fmt.Sprintf("task #%d status → %s", msg.id, msg.status)
		m.mode = modeNormal
		return m, m.loadTasks()

	case tea.KeyMsg:
		switch m.mode {
		case modePickTemplate:
			return m.updatePickTemplate(msg)
		case modeInputInstruction:
			return m.updateInputInstruction(msg)
		case modePickProject:
			return m.updatePickProject(msg)
		case modeInputTitle:
			return m.updateInputTitle(msg)
		case modeInputURL:
			return m.updateInputURL(msg)
		case modePickStatus:
			return m.updatePickStatus(msg)
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
	case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
		if taskID := m.selectedTaskID(); taskID > 0 {
			m.targetTaskID = taskID
			m.templates = m.loadTemplates()
			if len(m.templates) > 0 {
				m.mode = modePickTemplate
				m.templateCursor = 0
			}
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
		projects, err := m.database.ListProjects()
		if err == nil && len(projects) > 0 {
			m.projects = projects
			m.projectCursor = 0
			m.mode = modePickProject
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		if taskID := m.selectedTaskID(); taskID > 0 {
			allStatuses := []string{"open", "review", "done", "blocked", "archived"}
			var currentStatus string
			for _, pt := range m.trees {
				for _, tn := range pt.tasks {
					if tn.task.ID == taskID {
						currentStatus = tn.task.Status
					}
				}
			}
			var filtered []string
			for _, s := range allStatuses {
				if s != currentStatus {
					filtered = append(filtered, s)
				}
			}
			m.targetTaskID = taskID
			m.statuses = filtered
			m.statusCursor = 0
			m.mode = modePickStatus
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

func (m TasksModel) updatePickTemplate(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.templateCursor < len(m.templates)-1 {
			m.templateCursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.templateCursor > 0 {
			m.templateCursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		selected := m.templates[m.templateCursor]
		if selected == "implement" {
			m.mode = modeInputInstruction
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, textinput.Blink
		}
		return m, m.createAction(m.targetTaskID, selected, "{}")
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = modeNormal
	}
	return m, nil
}

func (m TasksModel) updateInputInstruction(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		instruction := m.textInput.Value()
		if instruction == "" {
			return m, nil
		}
		meta, _ := json.Marshal(map[string]string{"instruction": instruction})
		m.mode = modeNormal
		return m, m.createAction(m.targetTaskID, "implement", string(meta))
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m TasksModel) updatePickProject(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.projectCursor < len(m.projects)-1 {
			m.projectCursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.projectCursor > 0 {
			m.projectCursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		m.mode = modeInputTitle
		m.textInput.Placeholder = "タイトルを入力..."
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = modeNormal
	}
	return m, nil
}

func (m TasksModel) updateInputTitle(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		title := m.textInput.Value()
		if title == "" {
			return m, nil
		}
		m.newTaskTitle = title
		m.mode = modeInputURL
		m.textInput.Placeholder = "URL を入力 (空でスキップ)..."
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, textinput.Blink
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m TasksModel) updateInputURL(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		url := m.textInput.Value()
		proj := m.projects[m.projectCursor]
		m.mode = modeNormal
		return m, m.createTask(proj.ID, proj.Name, m.newTaskTitle, url)
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m TasksModel) updatePickStatus(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.statusCursor < len(m.statuses)-1 {
			m.statusCursor++
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.statusCursor > 0 {
			m.statusCursor--
		}
	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		selected := m.statuses[m.statusCursor]
		return m, m.updateTaskStatus(m.targetTaskID, selected)
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		m.mode = modeNormal
	}
	return m, nil
}

func (m TasksModel) updateViewDetail(msg tea.KeyMsg) (TasksModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q"))):
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

func (m TasksModel) updateTaskStatus(taskID int64, status string) tea.Cmd {
	return func() tea.Msg {
		if err := m.database.UpdateTask(taskID, status); err != nil {
			return taskUpdatedMsg{}
		}
		return taskUpdatedMsg{id: taskID, status: status}
	}
}

func (m TasksModel) createTask(projectID int64, projectName, title, url string) tea.Cmd {
	return func() tea.Msg {
		id, err := m.database.InsertTask(projectID, title, url, "{}")
		if err != nil {
			return taskCreatedMsg{}
		}
		return taskCreatedMsg{id: id, project: projectName, title: title}
	}
}

func (m TasksModel) createAction(taskID int64, templateID, meta string) tea.Cmd {
	return func() tea.Msg {
		id, err := m.database.InsertAction(templateID, &taskID, meta, "pending", "human")
		if err != nil {
			return actionCreatedMsg{}
		}
		return actionCreatedMsg{id: id, template: templateID, taskID: taskID}
	}
}

func (m TasksModel) selectedTaskID() int64 {
	if m.cursor >= 0 && m.cursor < len(m.lines) {
		return m.lines[m.cursor].taskID
	}
	return 0
}

func (m TasksModel) loadTemplates() []string {
	if m.tqDir == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(m.tqDir, "templates"))
	if err != nil {
		return nil
	}
	var templates []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".md") && name != "classify.md" {
			templates = append(templates, strings.TrimSuffix(name, ".md"))
		}
	}
	return templates
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
	switch m.mode {
	case modePickTemplate:
		return m.viewPickTemplate()
	case modeInputInstruction:
		return m.viewInputInstruction()
	case modePickProject:
		return m.viewPickProject()
	case modeInputTitle, modeInputURL:
		return m.viewInputTaskField()
	case modePickStatus:
		return m.viewPickStatus()
	case modeViewDetail:
		if m.detailAction != nil {
			return RenderDetailView(m.detailAction, m.detailScroll, m.width, m.height)
		}
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

func (m TasksModel) viewPickTemplate() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("  Add action to task #%d", m.targetTaskID)) + "\n\n")

	for i, t := range m.templates {
		prefix := "  "
		if i == m.templateCursor {
			prefix = "> "
			b.WriteString(styleTabActive.Render(prefix+t) + "\n")
		} else {
			b.WriteString(prefix + t + "\n")
		}
	}

	b.WriteString("\n" + styleHelp.Render("j/k: select  enter: confirm  esc: cancel"))
	return b.String()
}

func (m TasksModel) viewInputInstruction() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("  implement → task #%d", m.targetTaskID)) + "\n\n")
	b.WriteString("  " + m.textInput.View() + "\n\n")
	b.WriteString(styleHelp.Render("enter: confirm  esc: cancel"))
	return b.String()
}

func (m TasksModel) viewPickStatus() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("  Change status of task #%d", m.targetTaskID)) + "\n\n")

	for i, s := range m.statuses {
		st := StatusStyle(s)
		icon := StatusIcon(s)
		label := fmt.Sprintf("%s %s", icon, s)
		prefix := "  "
		if i == m.statusCursor {
			prefix = "> "
			b.WriteString(st.Bold(true).Render(prefix+label) + "\n")
		} else {
			b.WriteString(st.Render(prefix+label) + "\n")
		}
	}

	b.WriteString("\n" + styleHelp.Render("j/k: select  enter: confirm  esc: cancel"))
	return b.String()
}

func (m TasksModel) viewPickProject() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("  Create task — select project") + "\n\n")

	for i, p := range m.projects {
		prefix := "  "
		if i == m.projectCursor {
			prefix = "> "
			b.WriteString(styleTabActive.Render(prefix+p.Name) + "\n")
		} else {
			b.WriteString(prefix + p.Name + "\n")
		}
	}

	b.WriteString("\n" + styleHelp.Render("j/k: select  enter: confirm  esc: cancel"))
	return b.String()
}

func (m TasksModel) viewInputTaskField() string {
	var b strings.Builder
	proj := m.projects[m.projectCursor]

	if m.mode == modeInputTitle {
		b.WriteString(styleTitle.Render(fmt.Sprintf("  Create task — %s", proj.Name)) + "\n\n")
	} else {
		b.WriteString(styleTitle.Render(fmt.Sprintf("  Create task — %s: %s", proj.Name, m.newTaskTitle)) + "\n\n")
	}

	b.WriteString("  " + m.textInput.View() + "\n\n")
	b.WriteString(styleHelp.Render("enter: confirm  esc: cancel"))
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
