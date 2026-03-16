package dispatch

import (
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestCreateSelfImprovementAction(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateSelfImprovementAction(d, "/tmp/prompts", "my-prompt", []string{"allowed_tools", "timeout"})

	// Verify project was created
	p, err := d.GetProjectByName(selfImprovementProjectName)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != selfImprovementProjectName {
		t.Errorf("project name = %q, want %q", p.Name, selfImprovementProjectName)
	}

	// Verify action was created
	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].PromptID != selfImprovementPromptID {
		t.Errorf("prompt_id = %q, want %q", actions[0].PromptID, selfImprovementPromptID)
	}

	// Verify task work_dir was set
	task, err := d.GetTask(actions[0].TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.WorkDir != "/tmp/prompts" {
		t.Errorf("task work_dir = %q, want %q", task.WorkDir, "/tmp/prompts")
	}
}

func TestCreateSelfImprovementAction_NoDuplicateForSamePrompt(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateSelfImprovementAction(d, "/tmp/prompts", "my-prompt", []string{"timeout"})
	CreateSelfImprovementAction(d, "/tmp/prompts", "my-prompt", []string{"timeout"})

	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Errorf("expected 1 action (no duplicate), got %d", len(actions))
	}
}

func TestCreateSelfImprovementAction_DifferentPromptsGetSeparateActions(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateSelfImprovementAction(d, "/tmp/prompts", "prompt-a", []string{"timeout"})
	CreateSelfImprovementAction(d, "/tmp/prompts", "prompt-b", []string{"allowed_tools"})

	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 actions (one per prompt), got %d", len(actions))
	}
}

func TestCreateParseErrorFixAction(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateParseErrorFixAction(d, "/tmp/prompts", 42, "my-prompt", "invalid YAML")

	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].PromptID != parseErrorFixPromptID {
		t.Errorf("prompt_id = %q, want %q", actions[0].PromptID, parseErrorFixPromptID)
	}

	// Verify task work_dir was set
	task, err := d.GetTask(actions[0].TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.WorkDir != "/tmp/prompts" {
		t.Errorf("task work_dir = %q, want %q", task.WorkDir, "/tmp/prompts")
	}
}

func TestCreateParseErrorFixAction_SkipsForFixParseErrorPrompt(t *testing.T) {
	d := testutil.NewTestDB(t)

	// Should not create an action when the failing prompt is itself fix-parse-error
	CreateParseErrorFixAction(d, "/tmp/prompts", 42, parseErrorFixPromptID, "some error")

	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions (infinite loop prevention), got %d", len(actions))
	}
}

func TestCreateParseErrorFixAction_NoDuplicateForSameAction(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateParseErrorFixAction(d, "/tmp/prompts", 42, "my-prompt", "error 1")
	CreateParseErrorFixAction(d, "/tmp/prompts", 42, "my-prompt", "error 2")

	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Errorf("expected 1 action (no duplicate), got %d", len(actions))
	}
}

func TestCreateParseErrorFixAction_DifferentActionsGetSeparateFixActions(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateParseErrorFixAction(d, "/tmp/prompts", 42, "prompt-a", "error a")
	CreateParseErrorFixAction(d, "/tmp/prompts", 43, "prompt-b", "error b")

	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 actions (one per source action), got %d", len(actions))
	}
}
