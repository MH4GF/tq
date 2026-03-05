package dispatch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tmpl "github.com/MH4GF/tq/template"
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
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{
		AllowedTools: "Bash,Read",
		Timeout:      60,
	}

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

	wantArgs := []string{"-p", "do something", "--output-format", "json", "--allowedTools", "Bash,Read"}
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

	foundTQDir := false
	for _, e := range mock.GotEnv {
		if e == "TQ_DIR=/tmp/tq" {
			foundTQDir = true
			break
		}
	}
	if !foundTQDir {
		t.Errorf("env missing TQ_DIR=/tmp/tq, got %v", mock.GotEnv)
	}
}

func TestNonInteractiveWorker_Execute_Error(t *testing.T) {
	mock := &MockRunner{Err: errors.New("command failed")}
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{AllowedTools: "Bash", Timeout: 60}

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
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{AllowedTools: "Bash", Timeout: 30}

	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline, ok := mock.GotCtx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}

	expected := time.Now().Add(30 * time.Second)
	diff := deadline.Sub(expected)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("deadline diff from expected = %v, want within 2s", diff)
	}
}

func TestNonInteractiveWorker_Execute_Output(t *testing.T) {
	want := `{"status":"success","data":[1,2,3]}`
	wrapperJSON := `{"type":"result","subtype":"success","result":"{\"status\":\"success\",\"data\":[1,2,3]}"}`
	mock := &MockRunner{Output: []byte(wrapperJSON)}
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{AllowedTools: "Bash,Read,Edit", Timeout: 120}

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
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{AllowedTools: "Bash", Timeout: 60}

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
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{AllowedTools: "Bash", Timeout: 60}

	_, err := w.Execute(context.Background(), "fail", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "failed to parse claude JSON output") {
		t.Errorf("error = %q, want it to contain %q", got, "failed to parse claude JSON output")
	}
}

func TestNonInteractiveWorker_Execute_JSONSchema(t *testing.T) {
	envelope := `{"type":"result","subtype":"success","result":"some text","structured_output":{"task":{"id":0,"project_name":"works","title":"Test","url":"https://example.com"},"actions":[{"template_id":"check-pr-status"}]}}`
	mock := &MockRunner{Output: []byte(envelope)}
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	schema := `{"type":"object","properties":{"task":{"type":"object"}}}`
	cfg := tmpl.Config{
		AllowedTools: "Bash,Read",
		Timeout:      60,
		JSONSchema:   schema,
	}

	result, err := w.Execute(context.Background(), "classify", cfg, "/work", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, `"project_name":"works"`) {
		t.Errorf("result = %q, want to contain structured_output content", result)
	}

	// Verify --json-schema is in args
	foundSchema := false
	for i, a := range mock.GotArgs {
		if a == "--json-schema" {
			foundSchema = true
			if i+1 >= len(mock.GotArgs) {
				t.Fatal("--json-schema flag has no value")
			}
			if mock.GotArgs[i+1] != schema {
				t.Errorf("json-schema value = %q, want %q", mock.GotArgs[i+1], schema)
			}
			break
		}
	}
	if !foundSchema {
		t.Errorf("args missing --json-schema, got %v", mock.GotArgs)
	}
}

func TestNonInteractiveWorker_Execute_JSONSchema_TrimSpace(t *testing.T) {
	envelope := `{"type":"result","subtype":"success","result":"ok","structured_output":{"key":"value"}}`
	mock := &MockRunner{Output: []byte(envelope)}
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{
		AllowedTools: "Bash",
		Timeout:      60,
		JSONSchema:   "{\"type\":\"object\"}\n",
	}

	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, a := range mock.GotArgs {
		if a == "--json-schema" {
			if mock.GotArgs[i+1] != `{"type":"object"}` {
				t.Errorf("json-schema value = %q, want trimmed", mock.GotArgs[i+1])
			}
			break
		}
	}
}

func TestNonInteractiveWorker_Execute_JSONSchema_MissingStructuredOutput(t *testing.T) {
	envelope := `{"type":"result","subtype":"success","result":"some text"}`
	mock := &MockRunner{Output: []byte(envelope)}
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{
		AllowedTools: "Bash",
		Timeout:      60,
		JSONSchema:   `{"type":"object"}`,
	}

	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing structured_output") {
		t.Errorf("error = %q, want to contain 'missing structured_output'", err.Error())
	}
}

func TestNonInteractiveWorker_Execute_JSONSchema_ErrorSubtype(t *testing.T) {
	envelope := `{"type":"result","subtype":"error","result":"model refused"}`
	mock := &MockRunner{Output: []byte(envelope)}
	w := &NonInteractiveWorker{Runner: mock, TQDir: "/tmp/tq"}

	cfg := tmpl.Config{
		AllowedTools: "Bash",
		Timeout:      60,
		JSONSchema:   `{"type":"object"}`,
	}

	_, err := w.Execute(context.Background(), "test", cfg, "/work", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `subtype "error"`) {
		t.Errorf("error = %q, want to contain subtype error", err.Error())
	}
}
