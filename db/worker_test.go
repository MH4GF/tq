package db_test

import (
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestUpdateWorkerHeartbeat(t *testing.T) {
	d := testutil.NewTestDB(t)

	if err := d.UpdateWorkerHeartbeat(); err != nil {
		t.Fatalf("first heartbeat: %v", err)
	}

	if err := d.UpdateWorkerHeartbeat(); err != nil {
		t.Fatalf("second heartbeat (upsert): %v", err)
	}
}

func TestIsWorkerRunning(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, d *db.DB)
		threshold time.Duration
		want      bool
	}{
		{
			name:      "no heartbeat",
			setup:     func(t *testing.T, d *db.DB) {},
			threshold: 30 * time.Second,
			want:      false,
		},
		{
			name: "fresh heartbeat",
			setup: func(t *testing.T, d *db.DB) {
				t.Helper()
				if err := d.UpdateWorkerHeartbeat(); err != nil {
					t.Fatalf("update heartbeat: %v", err)
				}
			},
			threshold: 30 * time.Second,
			want:      true,
		},
		{
			name: "zero threshold treats as stale",
			setup: func(t *testing.T, d *db.DB) {
				t.Helper()
				if err := d.UpdateWorkerHeartbeat(); err != nil {
					t.Fatalf("update heartbeat: %v", err)
				}
			},
			threshold: 0,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			tt.setup(t, d)

			got, err := d.IsWorkerRunning(tt.threshold)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("IsWorkerRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}
