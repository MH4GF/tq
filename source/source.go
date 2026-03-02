package source

import "context"

// Notification represents a unified notification from any source.
type Notification struct {
	Source   string
	Message  string
	Metadata map[string]any
}

// Source abstracts a notification provider (GitHub, Gmail, Slack, etc.).
type Source interface {
	Name() string
	Fetch(ctx context.Context) ([]Notification, error)
	MarkProcessed(ctx context.Context, n Notification) error
}
