package db

import (
	"context"
	"time"
)

// CommandWriter defines all write operations.
type CommandWriter interface {
	// Action commands
	InsertAction(title string, taskID int64, metadata, status string) (int64, error)
	MarkDone(id int64, result string) error
	MarkFailed(id int64, result string) error
	MarkCancelled(id int64, result string) error
	MarkDispatched(id int64) error
	ResetToPending(id int64) error
	SetSessionInfo(id int64, sessionID, tmuxPane string) error
	MergeActionMetadata(id int64, updates map[string]any) error
	NextPending(ctx context.Context) (*Action, error)
	ClaimPending(ctx context.Context, id int64) (*Action, error)
	// Task commands
	InsertTask(projectID int64, title, metadata, workDir string) (int64, error)
	UpdateTask(id int64, status, reason string) error
	UpdateTaskProject(id, projectID int64) error
	UpdateTaskWorkDir(id int64, workDir string) error
	MergeTaskMetadata(id int64, updates map[string]any) error
	EnsureTask(projectID int64, title string) (int64, error)
	// Project commands
	InsertProject(name, workDir, metadata string) (int64, error)
	DeleteProject(id int64) error
	SetDispatchEnabled(projectID int64, enabled bool) error
	SetWorkDir(projectID int64, workDir string) error
	SetAllDispatchEnabled(enabled bool) error
	EnsureProject(name string) (int64, error)
	// Worker commands
	UpdateWorkerHeartbeat() error
	// Schedule commands
	InsertSchedule(taskID int64, instruction, title, cronExpr, metadata string) (int64, error)
	UpdateSchedule(id int64, title, cronExpr, metadata *string, taskID *int64) error
	UpdateScheduleEnabled(id int64, enabled bool) error
	DeleteSchedule(id int64) error
	UpdateScheduleLastRunAt(id int64, t string) error
}

// QueryReader defines all read operations.
type QueryReader interface {
	// Action queries
	GetAction(id int64) (*Action, error)
	ListActions(status string, taskID *int64, limit int) ([]Action, error)
	HasActiveActionWithMeta(taskID int64, metaKey, metaValue string) (bool, error)
	ListRunningInteractive() ([]Action, error)
	CountRunningInteractive() (int, error)
	CountByStatus() (map[string]int, error)
	ListActionsByTaskIDs(taskIDs []int64) (map[int64][]Action, error)
	// Task queries
	GetTask(id int64) (*Task, error)
	ListTasks(projectID int64, status string, limit int) ([]Task, error)
	ListTasksByProject(projectID int64) ([]Task, error)
	ListTasksByStatus(status string) ([]Task, error)
	GetOrCreateTriageTask(projectID int64) (int64, error)
	// Project queries
	GetProjectByID(id int64) (*Project, error)
	GetProjectByName(name string) (*Project, error)
	ListProjects(limit int) ([]Project, error)
	EnsureNotificationsProject() (int64, error)
	// Schedule queries
	GetSchedule(id int64) (*Schedule, error)
	ListSchedules(limit int) ([]Schedule, error)
	// Worker queries
	IsWorkerRunning(staleThreshold time.Duration) (bool, error)
	// Event queries
	ListEvents(entityType string, entityID int64) ([]Event, error)
	ListRecentEvents(limit int) ([]Event, error)
}

// Store implements both CommandWriter and QueryReader.
type Store interface {
	CommandWriter
	QueryReader
	Migrate() error
	Close() error
}

var _ Store = (*DB)(nil)
