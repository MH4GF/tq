package db_test

import (
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestGetSetting_Unset(t *testing.T) {
	d := testutil.NewTestDB(t)

	got, err := d.GetSetting(db.SettingDefaultMode)
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if got != "" {
		t.Errorf("unset key = %q, want empty string", got)
	}
}

func TestSetSetting_GetRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		writes []string // sequential SetSetting values; last one wins
		want   string
	}{
		{name: "single set", writes: []string{"experimental_bg"}, want: "experimental_bg"},
		{name: "overwrite", writes: []string{"interactive", "experimental_bg"}, want: "experimental_bg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)

			for _, v := range tc.writes {
				if err := d.SetSetting(db.SettingDefaultMode, v); err != nil {
					t.Fatalf("SetSetting %q: %v", v, err)
				}
			}

			got, err := d.GetSetting(db.SettingDefaultMode)
			if err != nil {
				t.Fatalf("GetSetting: %v", err)
			}
			if got != tc.want {
				t.Errorf("value = %q, want %q", got, tc.want)
			}
		})
	}
}
