package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestExecuteAction(t *testing.T) {
	tests := []struct {
		name              string
		promptMode        string
		workerResult      string
		workerErr         error
		beforeInteractive func(*db.Action) error
		wantMode          string
		wantStatus        string
		wantErrType       string // "failed", "deferred", or ""
		wantWorkerCount   int
	}{
		{
			name:            "noninteractive success",
			promptMode:      "noninteractive",
			workerResult:    `{"ok":true}`,
			wantMode:        ModeNonInteractive,
			wantStatus:      "done",
			wantWorkerCount: 1,
		},
		{
			name:            "noninteractive worker failure",
			promptMode:      "noninteractive",
			workerErr:       context.DeadlineExceeded,
			wantStatus:      "failed",
			wantErrType:     "failed",
			wantWorkerCount: 1,
		},
		{
			name:       "interactive deferred by BeforeInteractive",
			promptMode: "interactive",
			beforeInteractive: func(a *db.Action) error {
				return ErrInteractiveDeferred
			},
			wantStatus:      "pending",
			wantErrType:     "deferred",
			wantWorkerCount: 0,
		},
		{
			name:            "remote success",
			promptMode:      "remote",
			workerResult:    "remote:session=https://example.com/session/abc",
			wantMode:        ModeRemote,
			wantStatus:      "dispatched",
			wantWorkerCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			promptsDir := filepath.Join(t.TempDir(), "prompts")
			os.MkdirAll(promptsDir, 0o755)
			promptName := "test-" + tc.promptMode
			writeTestPromptWithMode(t, promptsDir, promptName, tc.promptMode, "")

			taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
			d.InsertAction(promptName, promptName, taskID, "{}", "pending")

			action, _ := d.NextPending(context.Background())

			worker := &countingWorker{result: tc.workerResult, err: tc.workerErr}
			workerFunc := func() Worker { return worker }

			result, err := ExecuteAction(context.Background(), ExecuteParams{
				DispatchConfig: DispatchConfig{
					DB:                 d,
					NonInteractiveFunc: workerFunc,
					InteractiveFunc:    workerFunc,
					RemoteFunc:         workerFunc,
				},
				PromptsDir:        promptsDir,
				BeforeInteractive: tc.beforeInteractive,
			}, action)

			switch tc.wantErrType {
			case "failed":
				var af *ActionFailedError
				if !errors.As(err, &af) {
					t.Fatalf("expected ActionFailedError, got %v", err)
				}
			case "deferred":
				if !errors.Is(err, ErrInteractiveDeferred) {
					t.Fatalf("expected ErrInteractiveDeferred, got %v", err)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result.Mode != tc.wantMode {
					t.Errorf("mode = %q, want %q", result.Mode, tc.wantMode)
				}
			}

			if worker.count != tc.wantWorkerCount {
				t.Errorf("worker.count = %d, want %d", worker.count, tc.wantWorkerCount)
			}

			a, _ := d.GetAction(action.ID)
			if a.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", a.Status, tc.wantStatus)
			}
		})
	}
}

func TestExecuteAction_PromptLoadError_CreatesFixAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	promptsDir := filepath.Join(t.TempDir(), "prompts")
	os.MkdirAll(promptsDir, 0o755)

	// Create action referencing a nonexistent prompt
	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	actionID, _ := d.InsertAction("test", "nonexistent-prompt", taskID, "{}", "pending")

	action, _ := d.GetAction(actionID)

	worker := &countingWorker{result: "ok"}
	workerFunc := func() Worker { return worker }

	_, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
		PromptsDir: promptsDir,
	}, action)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify a fix action was created
	actions, _ := d.ListActions(db.ActionStatusPending, nil)
	var fixAction *db.Action
	for i := range actions {
		if actions[i].PromptID == parseErrorFixPromptID {
			fixAction = &actions[i]
			break
		}
	}
	if fixAction == nil {
		t.Fatal("expected fix-parse-error action to be created")
	}

	var meta map[string]any
	json.Unmarshal([]byte(fixAction.Metadata), &meta)
	if meta["prompt_id"] != "nonexistent-prompt" {
		t.Errorf("meta prompt_id = %q, want %q", meta["prompt_id"], "nonexistent-prompt")
	}
}

func TestExecuteAction_RenderError_CreatesFixAction(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	promptsDir := filepath.Join(t.TempDir(), "prompts")
	os.MkdirAll(promptsDir, 0o755)

	// Write a prompt with a template that references a missing metadata key
	content := "---\ndescription: test\nmode: noninteractive\n---\n{{index .Action.Meta \"missing_key\"}}"
	os.WriteFile(filepath.Join(promptsDir, "bad-template.md"), []byte(content), 0o644)

	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	actionID, _ := d.InsertAction("test", "bad-template", taskID, "{}", "pending")

	action, _ := d.GetAction(actionID)

	worker := &countingWorker{result: "ok"}
	workerFunc := func() Worker { return worker }

	_, err := ExecuteAction(context.Background(), ExecuteParams{
		DispatchConfig: DispatchConfig{
			DB:                 d,
			NonInteractiveFunc: workerFunc,
			InteractiveFunc:    workerFunc,
			RemoteFunc:         workerFunc,
		},
		PromptsDir: promptsDir,
	}, action)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify a fix action was created
	actions, _ := d.ListActions(db.ActionStatusPending, nil)
	var fixAction *db.Action
	for i := range actions {
		if actions[i].PromptID == parseErrorFixPromptID {
			fixAction = &actions[i]
			break
		}
	}
	if fixAction == nil {
		t.Fatal("expected fix-parse-error action to be created")
	}
}

func TestExecuteAction_FixParseErrorPrompt_NoInfiniteLoop(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	promptsDir := filepath.Join(t.TempDir(), "prompts")
	os.MkdirAll(promptsDir, 0o755)

	// Create action with fix-parse-error prompt ID
	taskID, _ := d.InsertTask(1, "Test task", "https://example.com", "{}", "")
	actionID, _ := d.InsertAction("fix", parseErrorFixPromptID, taskID, `{"source_action_id":"1","prompt_id":"broken","error_message":"err"}`, "pending")

	// Verify CreateParseErrorFixAction skips when promptID is fix-parse-error
	CreateParseErrorFixAction(d, promptsDir, actionID, parseErrorFixPromptID, "some error")

	actions, _ := d.ListActions(db.ActionStatusPending, nil)
	fixCount := 0
	for _, a := range actions {
		// Skip the original action we inserted
		if a.ID == actionID {
			continue
		}
		if a.PromptID == parseErrorFixPromptID {
			fixCount++
		}
	}
	if fixCount != 0 {
		t.Errorf("expected 0 additional fix actions (infinite loop prevention), got %d", fixCount)
	}
}
