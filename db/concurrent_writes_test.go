package db_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestConcurrentWrites_FileBackedNoLockedErrors(t *testing.T) {
	d := testutil.NewFileTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, err := d.InsertTask(1, "concurrent stress", "{}", "")
	if err != nil {
		t.Fatal(err)
	}
	id, err := d.InsertAction("stress target", taskID, "{}", db.ActionStatusPending, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			key := fmt.Sprintf("k%d", idx)
			if err := d.MergeActionMetadata(id, map[string]any{key: idx}); err != nil {
				errs <- err
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	var collected []error
	for err := range errs {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		t.Fatalf("%d goroutines returned errors; first: %v", len(collected), collected[0])
	}
}
