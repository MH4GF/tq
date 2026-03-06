package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MH4GF/tq/prompt"
)

type MockRunner struct {
	Output  []byte
	Err     error
	GotName string
	GotArgs []string
	GotDir  string
	GotEnv  []string
	GotCtx  context.Context
}

func (m *MockRunner) Run(ctx context.Context, name string, args []string, dir string, env []string) ([]byte, error) {
	m.GotCtx = ctx
	m.GotName = name
	m.GotArgs = args
	m.GotDir = dir
	m.GotEnv = env
	return m.Output, m.Err
}

func TestNonInteractiveWorker_Execute(t *testing.T) {
	mock := &MockRunner{Output: []byte(`{"type":"result","subtype":"success","result":"{\"result\":\"ok\"}","cost_usd":0.01}`)}
	w := &NonInteractiveWorker{Runner: mock}

	cfg := prompt.Config{}

	result, err := w.Execute(context.Background(), "do something", cfg, "/work", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != `{"result":"ok"}` {
		t.Errorf("result = %q, want %q", result, `{"result":"ok"}`)
	}

	if mock.GotName != "claude" {
		t.Errorf("name = %q, want %q", mock.GotName, "claude")
	}

	wantArgs := []string{"-p", "do something", "--output-format", "json"}
	if len(mock.GotArgs) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d", len(mock.GotArgs), len(wantArgs))
	}
	for i, a := range wantArgs {
		if mock.GotArgs[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, mock.GotArgs[i], a)
		}
	}

	if mock.GotDir != "/work" {
		t.Errorf("dir = %q, want %q", mock.GotDir, "/work")
	}

	foundActionID := false
	for _, e := range mock.GotEnv {
		if e == "TQ_ACTION_ID=1" {
			foundActionID = true
			break
		}
	}
	if !foundActionID {
		t.Errorf("env missing TQ_ACTION_ID=1, got %v", mock.GotEnv)
	}
}

func TestNonInteractiveWorker_Execute_Error(t *testing.T) {
	mock := &MockRunner{Err: errors.New("command failed")}
	w := &NonInteractiveWorker{Runner: mock}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "fail", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "command failed" {
		t.Errorf("error = %q, want %q", err.Error(), "command failed")
	}
}

func TestNonInteractiveWorker_Execute_Timeout(t *testing.T) {
	mock := &MockRunner{Output: []byte(`{"type":"result","subtype":"success","result":"ok"}`)}
	w := &NonInteractiveWorker{Runner: mock}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline, ok := mock.GotCtx.Deadline()
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
	mock := &MockRunner{Output: []byte(wrapperJSON)}
	w := &NonInteractiveWorker{Runner: mock}

	cfg := prompt.Config{}

	got, err := w.Execute(context.Background(), "process data", cfg, "/projects/app", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestNonInteractiveWorker_Execute_ErrorSubtype(t *testing.T) {
	mock := &MockRunner{Output: []byte(`{"type":"result","subtype":"error","result":"model refused"}`)}
	w := &NonInteractiveWorker{Runner: mock}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "fail", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != `claude returned subtype "error": model refused` {
		t.Errorf("error = %q, want %q", got, `claude returned subtype "error": model refused`)
	}
}

func TestNonInteractiveWorker_Execute_MalformedJSON(t *testing.T) {
	mock := &MockRunner{Output: []byte(`not json at all`)}
	w := &NonInteractiveWorker{Runner: mock}

	cfg := prompt.Config{}

	_, err := w.Execute(context.Background(), "fail", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "failed to parse claude JSON output") {
		t.Errorf("error = %q, want it to contain %q", got, "failed to parse claude JSON output")
	}
}
