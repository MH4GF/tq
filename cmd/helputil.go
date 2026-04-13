package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/MH4GF/tq/db"
)

func tryOpenDBForHelp() (db.Store, func()) {
	if database != nil {
		return database, func() {}
	}
	dbPath, err := resolveDBPath()
	if err != nil {
		return nil, func() {}
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, func() {}
	}
	store, err := db.Open(dbPath)
	if err != nil {
		return nil, func() {}
	}
	return store, func() { _ = store.Close() }
}

func writeProjectHint(w io.Writer) {
	store, cleanup := tryOpenDBForHelp()
	defer cleanup()
	if store == nil {
		return
	}
	projects, err := store.ListProjects(20)
	if err != nil || len(projects) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "[agent hint] Available projects:")
	for _, p := range projects {
		_, _ = fmt.Fprintf(w, "  %d: %s\n", p.ID, p.Name)
	}
}
