package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LogEntry is a single line displayed in the TUI Activity section.
type LogEntry struct {
	Time    time.Time
	Message string
}

type logMsg LogEntry

func waitForLog(ch <-chan LogEntry) tea.Cmd {
	return func() tea.Msg {
		entry, ok := <-ch
		if !ok {
			return nil
		}
		return logMsg(entry)
	}
}

// TUILogHandler is a slog.Handler that sends log records to a channel.
type TUILogHandler struct {
	Ch    chan<- LogEntry
	Level slog.Leveler
}

func (h *TUILogHandler) Handle(_ context.Context, r slog.Record) error {
	msg := r.Message
	r.Attrs(func(a slog.Attr) bool {
		msg += " " + a.Key + "=" + fmt.Sprint(a.Value.Any())
		return true
	})
	select {
	case h.Ch <- LogEntry{Time: r.Time, Message: msg}:
	default:
	}
	return nil
}

func (h *TUILogHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.Level.Level()
}

func (h *TUILogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *TUILogHandler) WithGroup(_ string) slog.Handler      { return h }

// LogWriter is an io.Writer that sends written text to a channel as LogEntry.
type LogWriter struct {
	Ch chan<- LogEntry
}

func (w *LogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	select {
	case w.Ch <- LogEntry{Time: time.Now(), Message: msg}:
	default:
	}
	return len(p), nil
}
