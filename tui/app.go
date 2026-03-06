package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/MH4GF/tq/db"
)

type screen int

const (
	screenTasks screen = iota
	screenQueue
)

// BackgroundFunc is a function that runs in the background (ralph loop, watch, etc).
// It receives a context that is cancelled when the TUI exits.
type BackgroundFunc func(ctx context.Context) error

type Model struct {
	screen      screen
	queue       QueueModel
	tasks       TasksModel
	width       int
	height      int
	quitting    bool
	cancel      context.CancelFunc
	backgrounds []BackgroundFunc
	statusLine  string
	logCh       <-chan LogEntry
	logs        []LogEntry

	// Hierarchical navigation state
	selectedTaskID    int64
	selectedTaskTitle string
}

type tickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type backgroundStatusMsg struct {
	name string
	err  error
}

func New(database *db.DB, logCh <-chan LogEntry, backgrounds ...BackgroundFunc) Model {
	today := time.Now().Format("2006-01-02")
	return Model{
		screen:      screenTasks,
		queue:       NewQueueModel(database, today),
		tasks:       NewTasksModel(database, today),
		backgrounds: backgrounds,
		logCh:       logCh,
	}
}

func (m Model) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	cmds := []tea.Cmd{m.tasks.Init(), doTick()}

	if m.logCh != nil {
		cmds = append(cmds, waitForLog(m.logCh))
	}

	for _, bg := range m.backgrounds {
		bg := bg
		cmds = append(cmds, func() tea.Msg {
			err := bg(ctx)
			return backgroundStatusMsg{err: err}
		})
	}

	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := msg.Height - 14
		m.queue = m.queue.SetSize(msg.Width, contentHeight)
		m.tasks = m.tasks.SetSize(msg.Width, contentHeight)
		return m, nil

	case tickMsg:
		switch m.screen {
		case screenTasks:
			return m, tea.Batch(m.tasks.loadTasks(), doTick())
		case screenQueue:
			return m, tea.Batch(m.queue.loadActions(), doTick())
		}
		return m, doTick()

	case logMsg:
		m.logs = append(m.logs, LogEntry(msg))
		if len(m.logs) > 100 {
			m.logs = m.logs[len(m.logs)-100:]
		}
		return m, waitForLog(m.logCh)

	case backgroundStatusMsg:
		if msg.err != nil && msg.err != context.Canceled {
			m.statusLine = fmt.Sprintf("background error: %v", msg.err)
		}
		return m, nil

	case taskSelectedMsg:
		// Navigate from tasks to queue for the selected task
		m.screen = screenQueue
		m.selectedTaskID = msg.taskID
		m.selectedTaskTitle = msg.title
		taskID := msg.taskID
		m.queue = m.queue.SetTaskFilter(&taskID)
		return m, m.queue.loadActions()

	case tea.KeyMsg:
		// When queue is in detail view, delegate all keys to it
		if m.screen == screenQueue && m.queue.InDetailView() {
			var cmd tea.Cmd
			m.queue, cmd = m.queue.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("q"))):
			if m.screen == screenQueue {
				// Go back to tasks list
				m.screen = screenTasks
				m.selectedTaskID = 0
				m.selectedTaskTitle = ""
				return m, m.tasks.loadTasks()
			}
			// Quit from tasks screen
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.screen == screenQueue {
				m.screen = screenTasks
				m.selectedTaskID = 0
				m.selectedTaskTitle = ""
				return m, m.tasks.loadTasks()
			}
			return m, nil
		}
	}

	// Data messages go to their owning model regardless of active screen
	switch msg.(type) {
	case actionsLoadedMsg, actionUpdatedMsg:
		var cmd tea.Cmd
		m.queue, cmd = m.queue.Update(msg)
		return m, cmd
	case tasksLoadedMsg:
		var cmd tea.Cmd
		m.tasks, cmd = m.tasks.Update(msg)
		return m, cmd
	}

	// Key messages go to the active screen only
	var cmd tea.Cmd
	switch m.screen {
	case screenQueue:
		m.queue, cmd = m.queue.Update(msg)
	case screenTasks:
		m.tasks, cmd = m.tasks.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")

	switch m.screen {
	case screenTasks:
		b.WriteString(m.tasks.View())
	case screenQueue:
		b.WriteString(m.queue.View())
	}

	b.WriteString("\n")
	b.WriteString(m.renderActivity())
	if m.statusLine != "" {
		b.WriteString(styleWaitingHuman.Render(m.statusLine) + "\n")
	}
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderHeader() string {
	switch m.screen {
	case screenQueue:
		title := fmt.Sprintf("Tasks > #%d %s", m.selectedTaskID, m.selectedTaskTitle)
		return styleTabActive.Render(title)
	default:
		return styleTabActive.Render("Tasks")
	}
}

func (m Model) renderHelp() string {
	switch m.screen {
	case screenQueue:
		if m.queue.InDetailView() {
			return styleHelp.Render("j/k: scroll  q: back")
		}
		return styleHelp.Render("j/k: navigate  o: attach  v: view result  r: reload  q/esc: back")
	default:
		return styleHelp.Render("j/k: navigate  enter: select/expand  f: toggle focus  r: reload  q: quit")
	}
}

func (m Model) renderActivity() string {
	var b strings.Builder
	b.WriteString(styleMuted.Render("── Activity ──────────────────────") + "\n")

	start := 0
	if len(m.logs) > 9 {
		start = len(m.logs) - 9
	}
	shown := m.logs[start:]
	for _, e := range shown {
		ts := e.Time.Format("15:04")
		b.WriteString(styleMuted.Render("  "+ts+" ") + e.Message + "\n")
	}
	for i := len(shown); i < 9; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) Screen() screen {
	return m.screen
}

func (m Model) Queue() QueueModel {
	return m.queue
}

func (m Model) Tasks() TasksModel {
	return m.tasks
}

func (m Model) IsQuitting() bool {
	return m.quitting
}
