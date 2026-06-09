package db

import (
	"context"
	"time"
)

// CommandWriter defines all write operations.
type CommandWriter interface {
	// Action commands
	InsertAction(title string, taskID int64, metadata, status string, dispatchAfter *string, workDir string) (int64, error)
	InsertActionWithDependencies(spec ActionInsertSpec, deps []ActionDep) (int64, error)
	BulkInsertScheduledActions(specs []ActionInsertSpec, runs []ScheduleRunUpdate) ([]int64, error)
	MarkDone(id int64, result string) error
	MarkFailed(id int64, result string) error
	BulkMarkDone(updates []ActionDoneUpdate) error
	BulkMarkFailed(updates []ActionFailureUpdate) error
	MarkCancelled(id int64, result string) error
	MarkDispatched(id int64) error
	ResetToPending(id int64) error
	DeferToPending(id int64, retryAfter time.Duration) error
	MergeActionMetadata(id int64, updates map[string]any) error
	BulkMergeActionMetadata(updates []ActionMetadataMerge) error
	UpdateAction(id int64, title *string, taskID *int64, metadata, workDir, result *string) error
	NextPending(ctx context.Context) (*Action, error)
	ClaimPending(ctx context.Context, id int64) (*Action, error)
	ResumeAction(parentID int64, opts ResumeOptions) (int64, error)
	AddActionDependencies(actionID int64, deps []ActionDep) error
	ClearActionDependencies(actionID int64) error
	// Task commands
	InsertTask(projectID int64, title, metadata, workDir string) (int64, error)
	UpdateTaskFields(id int64, c TaskFieldChanges) error
	RecordTaskNote(taskID int64, kind, reason string, metadata map[string]any) error
	// Project commands
	InsertProject(name, workDir, metadata string) (int64, error)
	DeleteProject(id int64, cascade bool) error
	SetDispatchEnabled(projectID int64, enabled bool) error
	SetWorkDir(projectID int64, workDir string) error
	// Worker commands
	UpdateWorkerHeartbeat(maxInteractive int) error
	// Settings commands
	SetSetting(key, value string) error
	// Schedule commands
	InsertSchedule(taskID int64, instruction, title, cronExpr, metadata string) (int64, error)
	UpdateSchedule(id int64, title, cronExpr, metadata, instruction *string, taskID *int64) error
	UpdateScheduleEnabled(id int64, enabled bool) error
	DeleteSchedule(id int64) error
	BulkUpdateScheduleRuns(updates []ScheduleRunUpdate) error
}

// QueryReader defines all read operations.
type QueryReader interface {
	// Action queries
	GetAction(id int64) (*Action, error)
	ListActions(status string, taskID *int64, limit int) ([]Action, error)
	HasActiveActionsForSchedules(scheduleIDs []int64) (map[int64]bool, error)
	GetTaskActionCount(taskID int64, statuses []string) (int64, error)
	GetTasksByIDs(ids []int64) (map[int64]*Task, error)
	ListRunningWithDaemonShort() ([]Action, error)
	ListRunningOrphans(minAge time.Duration) ([]Action, error)
	CountRunningInteractive() (int, error)
	CountRunningNonInteractive() (int, error)
	CountPendingByDispatch() (PendingCounts, error)
	IsActionDispatchEnabled(actionID int64) (bool, error)
	ListActionsByTaskIDs(taskIDs []int64) (map[int64][]Action, error)
	ListActionsByTaskIDsForView(taskIDs []int64, dateFilter string) (map[int64][]Action, error)
	ListActionDependencies(actionID int64) ([]ActionDepStatus, error)
	ListActionDependenciesByActionIDs(ids []int64) (map[int64][]ActionDepStatus, error)
	// Task queries
	EnsureTaskOpenForAttach(taskID int64, op string) error
	GetTask(id int64) (*Task, error)
	ListTasks(projectID int64, status string, limit int) ([]Task, error)
	ListTasksByProjectIDs(projectIDs []int64) (map[int64][]Task, error)
	TaskStatusHistory(taskID int64) ([]TaskStatusHistoryEntry, error)
	TaskNotes(taskID int64, kindFilter string) ([]TaskNoteEntry, error)
	LatestTaskNotes(taskIDs []int64, kindFilter string) (map[int64]TaskNoteEntry, error)
	// Project queries
	GetProjectByID(id int64) (*Project, error)
	ListProjects(limit int) ([]Project, error)
	// Schedule queries
	GetSchedule(id int64) (*Schedule, error)
	ListSchedules(limit int) ([]Schedule, error)
	EnabledScheduleIDs(taskID int64) ([]int64, error)
	// Worker queries
	GetWorkerMaxInteractive(staleThreshold time.Duration) (int, error)
	// Settings queries
	GetSetting(key string) (string, error)
	ListSettings() (map[string]string, error)
	// Event queries
	ListEvents(entityType string, entityID int64) ([]Event, error)
	ListRecentEvents(limit int) ([]Event, error)
	// Search
	Search(keyword string, projectID int64) ([]SearchResult, error)
}

// TestHelper provides test-seam methods for test setup.
// These methods bypass validation and do not emit events.
type TestHelper interface {
	SetActionTmuxInfoForTest(id int64, tmuxSession, tmuxWindow *string, startedAt *time.Time) error
	SetScheduleTimestampsForTest(id int64, createdAt, lastRunAt *time.Time) error
	SetActionTimestampsForTest(id int64, createdAt, completedAt *time.Time) error
	SetTaskTimestampsForTest(id int64, createdAt, updatedAt *time.Time) error
	SetActionStatusForTest(id int64, status string) error
}

// Store implements both CommandWriter and QueryReader.
type Store interface {
	CommandWriter
	QueryReader
	TestHelper
	Migrate() error
	Close() error
}

var _ Store = (*DB)(nil)
