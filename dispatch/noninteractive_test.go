package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/prompt"
)

func TestNonInteractiveWorker_Execute(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"type":"result","subtype":"success","result":"{\"result\":\"ok\"}","cost_usd":0.01}`),
		failAt: -1,
	}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := prompt.Config{}

	result, err := w.Execute(context.Background(), "do something", cfg, "/work", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != `{"result":"ok"}` {
		t.Errorf("result = %q, want %q", result, `{"result":"ok"}`)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.calls))
	}

	c := runner.calls[0]
	if c.name != "claude" {
		t.Errorf("name = %q, want %q", c.name, "claude")
	}

	wantArgs := []string{"-p", "do something", "--output-format", "json"}
	if len(c.args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d", len(c.args), len(wantArgs))
	}
	for i, a := range wantArgs {
		if c.args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, c.args[i], a)
		}
	}

	if c.dir != "/work" {
		t.Errorf("dir = %q, want %q", c.dir, "/work")
	}

	foundActionID := false
	foundTaskID := false
	for _, e := range c.env {
		if e == "TQ_ACTION_ID=1" {
			foundActionID = true
		}
		if e == "TQ_TASK_ID=10" {
			foundTaskID = true
		}
	}
	if !foundActionID {
		t.Errorf("env missing TQ_ACTION_ID=1, got %v", c.env)
	}
	if !foundTaskID {
		t.Errorf("env missing TQ_TASK_ID=10, got %v", c.env)
	}
}

func TestNonInteractiveWorker_Execute_PermissionMode(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"type":"result","subtype":"success","result":"ok"}`),
		failAt: -1,
	}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := prompt.Config{PermissionMode: "plan"}

	_, err := w.Execute(context.Background(), "plan something", cfg, "/work", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := runner.calls[0]
	wantArgs := []string{"-p", "plan something", "--output-format", "json", "--permission-mode", "plan"}
	if len(c.args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d: %v", len(c.args), len(wantArgs), c.args)
	}
	for i, a := range wantArgs {
		if c.args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, c.args[i], a)
		}
	}
}

func TestNonInteractiveWorker_Execute_Error(t *testing.T) {
	runner := &mockRunner{err: errors.New("command failed"), failAt: 0}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "fail", cfg, "/work", 1, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "command failed" {
		t.Errorf("error = %q, want %q", err.Error(), "command failed")
	}
}

func TestNonInteractiveWorker_Execute_Timeout(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"type":"result","subtype":"success","result":"ok"}`),
		failAt: -1,
	}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline, ok := runner.calls[0].ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}

	expected := time.Now().Add(defaultTimeout * time.Second)
	diff := deadline.Sub(expected)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("deadline diff from expected = %v, want within 2s", diff)
	}
}

func TestNonInteractiveWorker_Execute_Output(t *testing.T) {
	want := `{"status":"success","data":[1,2,3]}`
	wrapperJSON := `{"type":"result","subtype":"success","result":"{\"status\":\"success\",\"data\":[1,2,3]}"}`
	runner := &mockRunner{output: []byte(wrapperJSON), failAt: -1}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := prompt.Config{}

	got, err := w.Execute(context.Background(), "process data", cfg, "/projects/app", 42, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNonInteractiveWorker_Execute_ErrorSubtype(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"type":"result","subtype":"error","result":"model refused"}`),
		failAt: -1,
	}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "fail", cfg, "/work", 1, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != `claude returned subtype "error": model refused` {
		t.Errorf("error = %q, want %q", got, `claude returned subtype "error": model refused`)
	}
}

func TestNonInteractiveWorker_Execute_MalformedJSON(t *testing.T) {
	runner := &mockRunner{output: []byte(`not json at all`), failAt: -1}
	w := &NonInteractiveWorker{Runner: runner}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "fail", cfg, "/work", 1, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "failed to parse claude JSON output") {
		t.Errorf("error = %q, want it to contain %q", got, "failed to parse claude JSON output")
	}
}
