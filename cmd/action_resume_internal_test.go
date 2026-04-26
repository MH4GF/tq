package cmd

import (
	"testing"

	"github.com/MH4GF/tq/dispatch"
)

func TestInteractiveWorkerForResume_PassesSession(t *testing.T) {
	cases := []struct {
		name    string
		session string
	}{
		{"main default", "main"},
		{"custom session", "foo"},
		{"empty falls through", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, ok := interactiveWorkerForResume(tc.session)().(*dispatch.InteractiveWorker)
			if !ok {
				t.Fatalf("factory did not return *dispatch.InteractiveWorker")
			}
			if w.Session != tc.session {
				t.Errorf("Session = %q, want %q", w.Session, tc.session)
			}
		})
	}
}

// Ensure the factory does not read package-level dispatchSession (which is set
// only by `tq action dispatch`). Setting dispatchSession to a sentinel must not
// leak into the worker built for resume.
func TestInteractiveWorkerForResume_IgnoresDispatchSession(t *testing.T) {
	prev := dispatchSession
	t.Cleanup(func() { dispatchSession = prev })

	dispatchSession = "should-not-leak"

	w, ok := interactiveWorkerForResume("foo")().(*dispatch.InteractiveWorker)
	if !ok {
		t.Fatalf("factory did not return *dispatch.InteractiveWorker")
	}
	if w.Session != "foo" {
		t.Errorf("Session = %q, want %q (dispatchSession leaked into resume factory)", w.Session, "foo")
	}
}
