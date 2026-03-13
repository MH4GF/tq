package dispatch

import (
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestCreateSelfImprovementAction(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateSelfImprovementAction(d, "my-prompt", []string{"allowed_tools", "timeout"})

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
}

func TestCreateSelfImprovementAction_NoDuplicateForSamePrompt(t *testing.T) {
	d := testutil.NewTestDB(t)

	CreateSelfImprovementAction(d, "my-prompt", []string{"timeout"})
	CreateSelfImprovementAction(d, "my-prompt", []string{"timeout"})

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

	CreateSelfImprovementAction(d, "prompt-a", []string{"timeout"})
	CreateSelfImprovementAction(d, "prompt-b", []string{"allowed_tools"})

	actions, err := d.ListActions(db.ActionStatusPending, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 actions (one per prompt), got %d", len(actions))
	}
}
