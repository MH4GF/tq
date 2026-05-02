package e2e_test

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/MH4GF/tq/cmd"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"tq": func() {
			cmd.ResetForTestscript()
			if err := cmd.Execute(); err != nil {
				os.Exit(1)
			}
		},
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			env.Setenv("TQ_DB_PATH", env.WorkDir+"/tq.db")
			env.Setenv("HOME", env.WorkDir)
			return nil
		},
	})
}
