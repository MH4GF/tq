package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	ghapi "github.com/google/go-github/v71/github"

	"github.com/MH4GF/tq/source"
)

func setupTestServer(t *testing.T, handler http.Handler) *GitHubSource {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	client := ghapi.NewClient(nil).WithAuthToken("test")
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL

	return &GitHubSource{
		client: client,
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode json: %v", err)
	}
}

// notificationJSON builds a raw notification payload for the list endpoint.
func notificationJSON(id, title, subjectURL, subjectType, reason, repoOwner, repoName string) map[string]any {
	return map[string]any{
		"id": id,
		"subject": map[string]any{
			"title": title,
			"url":   subjectURL,
			"type":  subjectType,
		},
		"reason": reason,
		"repository": map[string]any{
			"name": repoName,
			"owner": map[string]any{
				"login": repoOwner,
			},
		},
	}
}

func TestFetch_PullRequest(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		notifications := []map[string]any{
			notificationJSON("100", "Add feature", "https://api.github.com/repos/owner/repo/pulls/42", "PullRequest", "review_requested", "owner", "repo"),
		}
		writeJSON(t, w, notifications)
	})

	mux.HandleFunc("GET /repos/owner/repo/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		pr := map[string]any{
			"html_url":        "https://github.com/owner/repo/pull/42",
			"state":           "open",
			"merged":          false,
			"draft":           false,
			"mergeable_state": "clean",
			"user":            map[string]any{"login": "alice"},
			"labels":          []map[string]any{{"name": "enhancement"}},
			"assignees":       []map[string]any{{"login": "bob"}},
			"head":            map[string]any{"sha": "abc123"},
		}
		writeJSON(t, w, pr)
	})

	mux.HandleFunc("GET /repos/owner/repo/pulls/42/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []map[string]any{
			{"state": "APPROVED", "submitted_at": "2025-01-01T00:00:00Z"},
		}
		writeJSON(t, w, reviews)
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/abc123/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"state":    "success",
			"statuses": []map[string]any{{"state": "success"}},
		})
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/abc123/check-runs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"total_count": 1,
			"check_runs":  []map[string]any{{"status": "completed", "conclusion": "success"}},
		})
	})

	src := setupTestServer(t, mux)

	results, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	n := results[0]
	if n.Source != "gh-notification" {
		t.Errorf("source = %q, want %q", n.Source, "gh-notification")
	}
	if n.Metadata["state"] != "open" {
		t.Errorf("state = %v, want %q", n.Metadata["state"], "open")
	}
	if n.Metadata["author"] != "alice" {
		t.Errorf("author = %v, want %q", n.Metadata["author"], "alice")
	}
	if n.Metadata["approved"] != true {
		t.Errorf("approved = %v, want true", n.Metadata["approved"])
	}
	if n.Metadata["ci_passed"] != true {
		t.Errorf("ci_passed = %v, want true", n.Metadata["ci_passed"])
	}
	if n.Metadata["url"] != "https://github.com/owner/repo/pull/42" {
		t.Errorf("url = %v, want %q", n.Metadata["url"], "https://github.com/owner/repo/pull/42")
	}
}

func TestFetch_Issue(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		notifications := []map[string]any{
			notificationJSON("200", "Bug report", "https://api.github.com/repos/owner/repo/issues/10", "Issue", "comment", "owner", "repo"),
		}
		writeJSON(t, w, notifications)
	})

	mux.HandleFunc("GET /repos/owner/repo/issues/10", func(w http.ResponseWriter, r *http.Request) {
		issue := map[string]any{
			"html_url":  "https://github.com/owner/repo/issues/10",
			"state":     "open",
			"user":      map[string]any{"login": "carol"},
			"labels":    []map[string]any{{"name": "bug"}},
			"assignees": []map[string]any{{"login": "dave"}},
		}
		writeJSON(t, w, issue)
	})

	src := setupTestServer(t, mux)

	results, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	n := results[0]
	if n.Source != "gh-notification" {
		t.Errorf("source = %q, want %q", n.Source, "gh-notification")
	}
	if n.Metadata["state"] != "open" {
		t.Errorf("state = %v, want %q", n.Metadata["state"], "open")
	}
	if n.Metadata["author"] != "carol" {
		t.Errorf("author = %v, want %q", n.Metadata["author"], "carol")
	}
	if n.Metadata["url"] != "https://github.com/owner/repo/issues/10" {
		t.Errorf("url = %v, want %q", n.Metadata["url"], "https://github.com/owner/repo/issues/10")
	}
}

func TestFetch_MergedPRSkipped(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		notifications := []map[string]any{
			notificationJSON("300", "Merged PR", "https://api.github.com/repos/owner/repo/pulls/99", "PullRequest", "author", "owner", "repo"),
		}
		writeJSON(t, w, notifications)
	})

	mux.HandleFunc("GET /repos/owner/repo/pulls/99", func(w http.ResponseWriter, r *http.Request) {
		pr := map[string]any{
			"html_url":        "https://github.com/owner/repo/pull/99",
			"state":           "closed",
			"merged":          true,
			"draft":           false,
			"mergeable_state": "",
			"user":            map[string]any{"login": "alice"},
			"labels":          []any{},
			"assignees":       []any{},
			"head":            map[string]any{"sha": "def456"},
		}
		writeJSON(t, w, pr)
	})

	mux.HandleFunc("GET /repos/owner/repo/pulls/99/reviews", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, []any{})
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/def456/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"state": "success", "statuses": []any{}})
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/def456/check-runs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"total_count": 0, "check_runs": []any{}})
	})

	// Merged PRs should be marked as done
	threadDoneCalled := false
	mux.HandleFunc("DELETE /notifications/threads/300", func(w http.ResponseWriter, r *http.Request) {
		threadDoneCalled = true
		w.WriteHeader(http.StatusResetContent)
	})

	src := setupTestServer(t, mux)

	results, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for merged PR, got %d", len(results))
	}
	if !threadDoneCalled {
		t.Error("expected thread done to be called for merged PR")
	}
}

func TestFetch_ClosedIssueSkipped(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		notifications := []map[string]any{
			notificationJSON("400", "Closed issue", "https://api.github.com/repos/owner/repo/issues/5", "Issue", "state_change", "owner", "repo"),
		}
		writeJSON(t, w, notifications)
	})

	mux.HandleFunc("GET /repos/owner/repo/issues/5", func(w http.ResponseWriter, r *http.Request) {
		issue := map[string]any{
			"html_url":  "https://github.com/owner/repo/issues/5",
			"state":     "closed",
			"user":      map[string]any{"login": "eve"},
			"labels":    []any{},
			"assignees": []any{},
		}
		writeJSON(t, w, issue)
	})

	threadDoneCalled := false
	mux.HandleFunc("DELETE /notifications/threads/400", func(w http.ResponseWriter, r *http.Request) {
		threadDoneCalled = true
		w.WriteHeader(http.StatusResetContent)
	})

	src := setupTestServer(t, mux)

	results, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for closed issue, got %d", len(results))
	}
	if !threadDoneCalled {
		t.Error("expected thread done to be called for closed issue")
	}
}

func TestFetch_UnknownTypeSkipped(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		notifications := []map[string]any{
			notificationJSON("500", "Something", "https://api.github.com/repos/owner/repo/commits/abc", "Commit", "subscribed", "owner", "repo"),
		}
		writeJSON(t, w, notifications)
	})

	threadDoneCalled := false
	mux.HandleFunc("DELETE /notifications/threads/500", func(w http.ResponseWriter, r *http.Request) {
		threadDoneCalled = true
		w.WriteHeader(http.StatusResetContent)
	})

	src := setupTestServer(t, mux)

	results, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for unknown type, got %d", len(results))
	}
	if !threadDoneCalled {
		t.Error("expected thread done to be called for unknown type")
	}
}

func TestFetch_NotFound404(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		notifications := []map[string]any{
			notificationJSON("600", "Deleted issue", "https://api.github.com/repos/owner/repo/issues/999", "Issue", "mention", "owner", "repo"),
		}
		writeJSON(t, w, notifications)
	})

	mux.HandleFunc("GET /repos/owner/repo/issues/999", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		writeJSON(t, w, map[string]any{
			"message":           "Not Found",
			"documentation_url": "https://docs.github.com/rest",
		})
	})

	// enrichIssue returns nil on 404 without setting state/url in metadata.
	// isMergedOrClosed(m) returns false (no state), so the notification passes through
	// with partial metadata rather than being skipped.
	threadDoneCalled := false
	mux.HandleFunc("DELETE /notifications/threads/600", func(w http.ResponseWriter, r *http.Request) {
		threadDoneCalled = true
		w.WriteHeader(http.StatusResetContent)
	})

	src := setupTestServer(t, mux)

	results, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	// 404 returns nil from enrichIssue (no error), state not set, so notification passes through
	if len(results) != 1 {
		t.Fatalf("expected 1 result (404 does not cause error), got %d", len(results))
	}
	if threadDoneCalled {
		t.Error("thread done should not be called for 404 (notification passes through)")
	}
}

func TestMarkProcessed(t *testing.T) {
	mux := http.NewServeMux()

	deleteCalled := false
	mux.HandleFunc("DELETE /notifications/threads/789", func(w http.ResponseWriter, r *http.Request) {
		deleteCalled = true
		w.WriteHeader(http.StatusResetContent)
	})

	src := setupTestServer(t, mux)

	n := source.Notification{
		Source:  "gh-notification",
		Message: "test",
		Metadata: map[string]any{
			"id": "789",
		},
	}

	if err := src.MarkProcessed(context.Background(), n); err != nil {
		t.Fatalf("MarkProcessed returned error: %v", err)
	}
	if !deleteCalled {
		t.Error("expected DELETE /notifications/threads/789 to be called")
	}
}

func TestMarkProcessed_MissingID(t *testing.T) {
	src := &GitHubSource{
		client: ghapi.NewClient(nil),
	}

	n := source.Notification{
		Metadata: map[string]any{},
	}

	err := src.MarkProcessed(context.Background(), n)
	if err == nil {
		t.Fatal("expected error for missing thread id")
	}
}

func TestBuildMessage_PullRequest(t *testing.T) {
	m := map[string]any{
		"reason":       "review_requested",
		"title":        "Add feature",
		"repo":         "owner/repo",
		"subject_type": "PullRequest",
		"url":          "https://github.com/owner/repo/pull/42",
		"approved":     true,
		"ci_passed":    true,
		"ci_failed":    false,
		"mergeable":    "clean",
	}

	msg := buildMessage(m)

	// ci_failed=false is not included (only shown when true)
	expected := "[GitHub通知] レビュー依頼: Add feature (owner/repo) | https://github.com/owner/repo/pull/42 | approved=true, ci_passed=true, mergeable=clean"
	if msg != expected {
		t.Errorf("buildMessage =\n  %q\nwant\n  %q", msg, expected)
	}
}

func TestBuildMessage_PullRequest_CIFailed(t *testing.T) {
	m := map[string]any{
		"reason":       "ci_activity",
		"title":        "Broken build",
		"repo":         "owner/repo",
		"subject_type": "PullRequest",
		"url":          "https://github.com/owner/repo/pull/5",
		"approved":     false,
		"ci_passed":    false,
		"ci_failed":    true,
		"mergeable":    "blocked",
	}

	msg := buildMessage(m)

	expected := "[GitHub通知] CI: Broken build (owner/repo) | https://github.com/owner/repo/pull/5 | approved=false, ci_passed=false, ci_failed=true, mergeable=blocked"
	if msg != expected {
		t.Errorf("buildMessage =\n  %q\nwant\n  %q", msg, expected)
	}
}

func TestBuildMessage_PullRequest_NoCIFailed(t *testing.T) {
	m := map[string]any{
		"reason":       "comment",
		"title":        "Fix bug",
		"repo":         "owner/repo",
		"subject_type": "PullRequest",
		"url":          "https://github.com/owner/repo/pull/10",
		"approved":     false,
		"ci_passed":    false,
		"ci_failed":    false,
		"mergeable":    "blocked",
	}

	msg := buildMessage(m)

	// ci_failed=false should NOT appear (only shown when true)
	expected := "[GitHub通知] コメント: Fix bug (owner/repo) | https://github.com/owner/repo/pull/10 | approved=false, ci_passed=false, mergeable=blocked"
	if msg != expected {
		t.Errorf("buildMessage =\n  %q\nwant\n  %q", msg, expected)
	}
}

func TestBuildMessage_Issue(t *testing.T) {
	m := map[string]any{
		"reason":       "mention",
		"title":        "Bug report",
		"repo":         "owner/repo",
		"subject_type": "Issue",
		"url":          "https://github.com/owner/repo/issues/10",
	}

	msg := buildMessage(m)

	expected := "[GitHub通知] メンション: Bug report (owner/repo) | https://github.com/owner/repo/issues/10"
	if msg != expected {
		t.Errorf("buildMessage =\n  %q\nwant\n  %q", msg, expected)
	}
}

func TestBuildMessage_NoURL(t *testing.T) {
	m := map[string]any{
		"reason":       "subscribed",
		"title":        "Some notification",
		"repo":         "owner/repo",
		"subject_type": "Release",
	}

	msg := buildMessage(m)

	expected := "[GitHub通知] 購読: Some notification (owner/repo)"
	if msg != expected {
		t.Errorf("buildMessage =\n  %q\nwant\n  %q", msg, expected)
	}
}

func TestIsMergedOrClosed(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		want bool
	}{
		{
			name: "merged PR",
			m:    map[string]any{"merged": true, "state": "closed"},
			want: true,
		},
		{
			name: "closed issue",
			m:    map[string]any{"state": "closed"},
			want: true,
		},
		{
			name: "open PR",
			m:    map[string]any{"merged": false, "state": "open"},
			want: false,
		},
		{
			name: "no state set",
			m:    map[string]any{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMergedOrClosed(tt.m)
			if got != tt.want {
				t.Errorf("isMergedOrClosed(%v) = %v, want %v", tt.m, got, tt.want)
			}
		})
	}
}

func TestReasonLabel(t *testing.T) {
	tests := []struct {
		reason string
		want   string
	}{
		{"review_requested", "レビュー依頼"},
		{"comment", "コメント"},
		{"mention", "メンション"},
		{"ci_activity", "CI"},
		{"state_change", "状態変更"},
		{"assign", "アサイン"},
		{"author", "作成者通知"},
		{"subscribed", "購読"},
		{"unknown_reason", "unknown_reason"},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := reasonLabel(tt.reason)
			if got != tt.want {
				t.Errorf("reasonLabel(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
	}
}

func TestLabelNames(t *testing.T) {
	labels := []*ghapi.Label{
		{Name: ghapi.Ptr("bug")},
		{Name: ghapi.Ptr("enhancement")},
	}
	got := labelNames(labels)
	if len(got) != 2 || got[0] != "bug" || got[1] != "enhancement" {
		t.Errorf("labelNames = %v, want [bug enhancement]", got)
	}
}

func TestLabelNames_Empty(t *testing.T) {
	got := labelNames(nil)
	if len(got) != 0 {
		t.Errorf("labelNames(nil) = %v, want []", got)
	}
}

func TestUserLogins(t *testing.T) {
	users := []*ghapi.User{
		{Login: ghapi.Ptr("alice")},
		{Login: ghapi.Ptr("bob")},
	}
	got := userLogins(users)
	if len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Errorf("userLogins = %v, want [alice bob]", got)
	}
}

func TestFetch_MultipleNotificationsMixed(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		notifications := []map[string]any{
			notificationJSON("1", "Open PR", "https://api.github.com/repos/owner/repo/pulls/1", "PullRequest", "review_requested", "owner", "repo"),
			notificationJSON("2", "Open issue", "https://api.github.com/repos/owner/repo/issues/2", "Issue", "comment", "owner", "repo"),
			notificationJSON("3", "Closed issue", "https://api.github.com/repos/owner/repo/issues/3", "Issue", "state_change", "owner", "repo"),
		}
		writeJSON(t, w, notifications)
	})

	// Open PR enrichment
	mux.HandleFunc("GET /repos/owner/repo/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"html_url": "https://github.com/owner/repo/pull/1", "state": "open", "merged": false,
			"draft": false, "mergeable_state": "clean", "user": map[string]any{"login": "a"},
			"labels": []any{}, "assignees": []any{}, "head": map[string]any{"sha": "sha1"},
		})
	})
	mux.HandleFunc("GET /repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, []any{})
	})
	mux.HandleFunc("GET /repos/owner/repo/commits/sha1/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"state": "success", "statuses": []any{}})
	})
	mux.HandleFunc("GET /repos/owner/repo/commits/sha1/check-runs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"total_count": 0, "check_runs": []any{}})
	})

	// Open issue enrichment
	mux.HandleFunc("GET /repos/owner/repo/issues/2", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"html_url": "https://github.com/owner/repo/issues/2", "state": "open",
			"user": map[string]any{"login": "b"}, "labels": []any{}, "assignees": []any{},
		})
	})

	// Closed issue enrichment
	mux.HandleFunc("GET /repos/owner/repo/issues/3", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"html_url": "https://github.com/owner/repo/issues/3", "state": "closed",
			"user": map[string]any{"login": "c"}, "labels": []any{}, "assignees": []any{},
		})
	})

	mux.HandleFunc("DELETE /notifications/threads/3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusResetContent)
	})

	src := setupTestServer(t, mux)

	results, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 active results, got %d", len(results))
	}

	// Verify both active notifications are present (order may vary due to goroutines)
	sources := map[string]bool{}
	for _, r := range results {
		sources[fmt.Sprintf("%v", r.Metadata["id"])] = true
	}
	if !sources["1"] {
		t.Error("missing notification id=1 (open PR)")
	}
	if !sources["2"] {
		t.Error("missing notification id=2 (open issue)")
	}
}
