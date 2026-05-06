// Package e2e_test runs end-to-end CLI scenarios via testscript.
//
// Each scenario lives in testdata/script/*.txtar and is executed
// against the in-process tq binary registered in TestMain. The Setup
// hook isolates DB, HOME, and tmux socket directories under $WORK
// so scenarios can run in parallel without interference.
package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/MH4GF/tq/cmd"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"tq": func() {
			cmd.ResetState()
			if err := cmd.Execute(); err != nil {
				os.Exit(1)
			}
		},
	})
}

func TestE2E(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Setenv("TQ_DB_URL", filepath.Join(env.WorkDir, "tq.db"))
			env.Setenv("HOME", env.WorkDir)

			// tmux unix sockets are constrained by the platform's
			// sun_path limit (~104 bytes on macOS). $WORK lives under
			// a deep testscript path, so allocate the tmux tmpdir
			// directly under /tmp to keep the socket path short.
			tmuxDir, err := os.MkdirTemp("/tmp", "tq-e2e-tmux-")
			if err != nil {
				return err
			}
			env.Setenv("TMUX_TMPDIR", tmuxDir)
			env.Defer(func() { _ = os.RemoveAll(tmuxDir) })
			return nil
		},
	})
}
