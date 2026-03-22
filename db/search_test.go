package db_test

import (
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestExtractSnippet(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		keyword      string
		contextChars int
		want         string
	}{
		{
			name:         "short string fully shown",
			value:        "Fix login bug",
			keyword:      "login",
			contextChars: 40,
			want:         "Fix login bug",
		},
		{
			name:         "long string with context",
			value:        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAFIX login bugBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			keyword:      "login",
			contextChars: 10,
			want:         "...AAAAAAFIX login bugBBBBBB...",
		},
		{
			name:         "case insensitive match",
			value:        "Fix LOGIN Bug",
			keyword:      "login",
			contextChars: 40,
			want:         "Fix LOGIN Bug",
		},
		{
			name:         "newlines replaced with spaces",
			value:        "line1\nlogin\nline3",
			keyword:      "login",
			contextChars: 40,
			want:         "line1 login line3",
		},
		{
			name:         "no match returns truncated",
			value:        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			keyword:      "notfound",
			contextChars: 10,
			want:         "AAAAAAAAAAAAAAAAAAAA...",
		},
		{
			name:         "multibyte characters not corrupted at boundary",
			value:        "あいうえおかきくけこさしすせそログインたちつてとなにぬねのはひふへほ",
			keyword:      "ログイン",
			contextChars: 5,
			want:         "...さしすせそログインたちつてと...",
		},
		{
			name:         "multibyte no match truncated cleanly",
			value:        "あいうえおかきくけこさしすせそたちつてとなにぬねの",
			keyword:      "notfound",
			contextChars: 5,
			want:         "あいうえおかきくけこ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.ExportExtractSnippet(tt.value, tt.keyword, tt.contextChars)
			if got != tt.want {
				t.Errorf("extractSnippet() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		setup   func(d *db.DB)
		wantLen int
		check   func(t *testing.T, results []db.SearchResult)
	}{
		{
			name:    "match task title",
			keyword: "login",
			setup: func(d *db.DB) {
				d.InsertTask(1, "Fix login bug", "{}", "")
			},
			wantLen: 1,
			check: func(t *testing.T, results []db.SearchResult) {
				t.Helper()
				r := results[0]
				if r.EntityType != "task" {
					t.Errorf("entity_type = %q, want task", r.EntityType)
				}
				if r.Field != "title" {
					t.Errorf("field = %q, want title", r.Field)
				}
			},
		},
		{
			name:    "match task metadata",
			keyword: "example.com",
			setup: func(d *db.DB) {
				d.InsertTask(1, "Some task", `{"url":"https://example.com/pr/1"}`, "")
			},
			wantLen: 1,
			check: func(t *testing.T, results []db.SearchResult) {
				t.Helper()
				if results[0].Field != "metadata" {
					t.Errorf("field = %q, want metadata", results[0].Field)
				}
			},
		},
		{
			name:    "match action title",
			keyword: "deploy",
			setup: func(d *db.DB) {
				taskID, _ := d.InsertTask(1, "Release", "{}", "")
				d.InsertAction("deploy to prod", taskID, "{}", db.ActionStatusPending)
			},
			wantLen: 1,
			check: func(t *testing.T, results []db.SearchResult) {
				t.Helper()
				if results[0].EntityType != "action" {
					t.Errorf("entity_type = %q, want action", results[0].EntityType)
				}
				if results[0].Field != "title" {
					t.Errorf("field = %q, want title", results[0].Field)
				}
			},
		},
		{
			name:    "match action result",
			keyword: "resolved",
			setup: func(d *db.DB) {
				taskID, _ := d.InsertTask(1, "Bug fix", "{}", "")
				actionID, _ := d.InsertAction("fix", taskID, "{}", db.ActionStatusPending)
				d.MarkDone(actionID, "resolved the login issue")
			},
			wantLen: 1,
			check: func(t *testing.T, results []db.SearchResult) {
				t.Helper()
				if results[0].Field != "result" {
					t.Errorf("field = %q, want result", results[0].Field)
				}
			},
		},
		{
			name:    "match action metadata",
			keyword: "pr_url",
			setup: func(d *db.DB) {
				taskID, _ := d.InsertTask(1, "Review", "{}", "")
				d.InsertAction("review", taskID, `{"pr_url":"https://github.com/foo/bar/pull/1"}`, db.ActionStatusPending)
			},
			wantLen: 1,
			check: func(t *testing.T, results []db.SearchResult) {
				t.Helper()
				if results[0].Field != "metadata" {
					t.Errorf("field = %q, want metadata", results[0].Field)
				}
			},
		},
		{
			name:    "no match",
			keyword: "nonexistent",
			setup: func(d *db.DB) {
				d.InsertTask(1, "Hello world", "{}", "")
			},
			wantLen: 0,
		},
		{
			name:    "case insensitive",
			keyword: "LOGIN",
			setup: func(d *db.DB) {
				d.InsertTask(1, "Fix login bug", "{}", "")
			},
			wantLen: 1,
		},
		{
			name:    "multiple matches across entities",
			keyword: "auth",
			setup: func(d *db.DB) {
				taskID, _ := d.InsertTask(1, "Auth refactor", "{}", "")
				d.InsertAction("update auth module", taskID, "{}", db.ActionStatusPending)
			},
			wantLen: 2,
		},
		{
			name:    "empty keyword returns empty",
			keyword: "",
			setup: func(d *db.DB) {
				d.InsertTask(1, "Something", "{}", "")
			},
			wantLen: 0,
		},
		{
			name:    "whitespace-only keyword returns empty",
			keyword: "   ",
			setup: func(d *db.DB) {
				d.InsertTask(1, "Something", "{}", "")
			},
			wantLen: 0,
		},
		{
			name:    "LIKE wildcards in keyword are escaped",
			keyword: "100%",
			setup: func(d *db.DB) {
				d.InsertTask(1, "100% complete", "{}", "")
				d.InsertTask(1, "1000 items", "{}", "")
			},
			wantLen: 1,
			check: func(t *testing.T, results []db.SearchResult) {
				t.Helper()
				if results[0].Snippet != "100% complete" {
					t.Errorf("snippet = %q, want %q", results[0].Snippet, "100% complete")
				}
			},
		},
		{
			name:    "underscore in keyword is escaped",
			keyword: "foo_bar",
			setup: func(d *db.DB) {
				d.InsertTask(1, "foo_bar_baz", "{}", "")
				d.InsertTask(1, "fooXbar", "{}", "")
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			tt.setup(d)

			results, err := d.Search(tt.keyword)
			if err != nil {
				t.Fatalf("Search() error: %v", err)
			}
			if len(results) != tt.wantLen {
				t.Fatalf("Search() returned %d results, want %d", len(results), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, results)
			}
		})
	}
}
