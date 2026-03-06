package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	ghapi "github.com/google/go-github/v71/github"
)

func TestEnrichPullRequest(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/owner/repo/pulls/42", func(w http.ResponseWriter, r *http.Request) {
		pr := ghapi.PullRequest{
			HTMLURL:        ghapi.Ptr("https://github.com/owner/repo/pull/42"),
			State:          ghapi.Ptr("open"),
			User:           &ghapi.User{Login: ghapi.Ptr("author1")},
			Merged:         ghapi.Ptr(false),
			Draft:          ghapi.Ptr(false),
			MergeableState: ghapi.Ptr("clean"),
			Labels:         []*ghapi.Label{{Name: ghapi.Ptr("bug")}},
			Assignees:      []*ghapi.User{{Login: ghapi.Ptr("reviewer1")}},
			Head:           &ghapi.PullRequestBranch{SHA: ghapi.Ptr("abc123"), Ref: ghapi.Ptr("feature-branch")},
		}
		json.NewEncoder(w).Encode(pr)
	})

	mux.HandleFunc("GET /repos/owner/repo/pulls/42/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*ghapi.PullRequestReview{
			{
				State:       ghapi.Ptr("APPROVED"),
				SubmittedAt: &ghapi.Timestamp{},
			},
		}
		json.NewEncoder(w).Encode(reviews)
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/abc123/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.CombinedStatus{
			Statuses: []*ghapi.RepoStatus{
				{State: ghapi.Ptr("success")},
			},
		})
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/abc123/check-runs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.ListCheckRunsResults{
			Total:     ghapi.Ptr(1),
			CheckRuns: []*ghapi.CheckRun{{Status: ghapi.Ptr("completed"), Conclusion: ghapi.Ptr("success")}},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghapi.NewClient(nil).WithAuthToken("test")
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL

	s := &GitHubSource{client: client}

	m := map[string]any{
		"id":           "1",
		"reason":       "review_requested",
		"subject_type": "PullRequest",
		"repo":         "owner/repo",
		"title":        "Fix bug",
	}

	subjectURL, _ := url.Parse(ts.URL + "/repos/owner/repo/pulls/42")
	err := s.enrichPullRequest(context.Background(), subjectURL, "owner", "repo", m)
	if err != nil {
		t.Fatalf("enrichPullRequest: %v", err)
	}

	if m["url"] != "https://github.com/owner/repo/pull/42" {
		t.Errorf("url = %v, want https://github.com/owner/repo/pull/42", m["url"])
	}
	if m["head_branch"] != "feature-branch" {
		t.Errorf("head_branch = %v, want feature-branch", m["head_branch"])
	}
	if m["state"] != "open" {
		t.Errorf("state = %v, want open", m["state"])
	}
	if m["author"] != "author1" {
		t.Errorf("author = %v, want author1", m["author"])
	}
	if m["merged"] != false {
		t.Errorf("merged = %v, want false", m["merged"])
	}
	if m["approved"] != true {
		t.Errorf("approved = %v, want true", m["approved"])
	}
	if m["review_decision"] != "APPROVED" {
		t.Errorf("review_decision = %v, want APPROVED", m["review_decision"])
	}
	if m["ci_passed"] != true {
		t.Errorf("ci_passed = %v, want true", m["ci_passed"])
	}
	if m["ci_failed"] != false {
		t.Errorf("ci_failed = %v, want false", m["ci_failed"])
	}
}

func TestEnrichPullRequest_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/pulls/99", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ghapi.ErrorResponse{
			Response: &http.Response{StatusCode: http.StatusNotFound},
			Message:  "Not Found",
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghapi.NewClient(nil).WithAuthToken("test")
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL

	s := &GitHubSource{client: client}
	m := map[string]any{}

	subjectURL, _ := url.Parse(ts.URL + "/repos/owner/repo/pulls/99")
	err := s.enrichPullRequest(context.Background(), subjectURL, "owner", "repo", m)
	if err != nil {
		t.Fatalf("expected nil error for 404, got: %v", err)
	}
}

func TestCheckCIStatus_AllSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r/commits/sha1/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.CombinedStatus{
			Statuses: []*ghapi.RepoStatus{
				{State: ghapi.Ptr("success")},
			},
		})
	})
	mux.HandleFunc("GET /repos/o/r/commits/sha1/check-runs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.ListCheckRunsResults{
			Total:     ghapi.Ptr(2),
			CheckRuns: []*ghapi.CheckRun{
				{Status: ghapi.Ptr("completed"), Conclusion: ghapi.Ptr("success")},
				{Status: ghapi.Ptr("completed"), Conclusion: ghapi.Ptr("skipped")},
			},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghapi.NewClient(nil).WithAuthToken("test")
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL

	s := &GitHubSource{client: client}
	passed, failed := s.checkCIStatus(context.Background(), "o", "r", "sha1")
	if !passed {
		t.Error("expected passed=true")
	}
	if failed {
		t.Error("expected failed=false")
	}
}

func TestCheckCIStatus_WithFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r/commits/sha2/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.CombinedStatus{
			Statuses: []*ghapi.RepoStatus{
				{State: ghapi.Ptr("failure")},
			},
		})
	})
	mux.HandleFunc("GET /repos/o/r/commits/sha2/check-runs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.ListCheckRunsResults{
			Total:     ghapi.Ptr(1),
			CheckRuns: []*ghapi.CheckRun{
				{Status: ghapi.Ptr("completed"), Conclusion: ghapi.Ptr("failure")},
			},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghapi.NewClient(nil).WithAuthToken("test")
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL

	s := &GitHubSource{client: client}
	passed, failed := s.checkCIStatus(context.Background(), "o", "r", "sha2")
	if passed {
		t.Error("expected passed=false")
	}
	if !failed {
		t.Error("expected failed=true")
	}
}

func TestCheckCIStatus_Pending(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r/commits/sha3/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.CombinedStatus{Statuses: []*ghapi.RepoStatus{}})
	})
	mux.HandleFunc("GET /repos/o/r/commits/sha3/check-runs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.ListCheckRunsResults{
			Total:     ghapi.Ptr(1),
			CheckRuns: []*ghapi.CheckRun{
				{Status: ghapi.Ptr("in_progress")},
			},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghapi.NewClient(nil).WithAuthToken("test")
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL

	s := &GitHubSource{client: client}
	passed, failed := s.checkCIStatus(context.Background(), "o", "r", "sha3")
	if passed {
		t.Error("expected passed=false for in_progress")
	}
	if failed {
		t.Error("expected failed=false for in_progress")
	}
}

func TestEnrichPullRequest_ChangesRequested(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/owner/repo/pulls/10", func(w http.ResponseWriter, r *http.Request) {
		pr := ghapi.PullRequest{
			HTMLURL:        ghapi.Ptr("https://github.com/owner/repo/pull/10"),
			State:          ghapi.Ptr("open"),
			User:           &ghapi.User{Login: ghapi.Ptr("dev")},
			Merged:         ghapi.Ptr(false),
			Draft:          ghapi.Ptr(true),
			MergeableState: ghapi.Ptr("dirty"),
			Labels:         []*ghapi.Label{},
			Assignees:      []*ghapi.User{},
			Head:           &ghapi.PullRequestBranch{SHA: ghapi.Ptr("def456")},
		}
		json.NewEncoder(w).Encode(pr)
	})

	mux.HandleFunc("GET /repos/owner/repo/pulls/10/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*ghapi.PullRequestReview{
			{State: ghapi.Ptr("APPROVED"), SubmittedAt: &ghapi.Timestamp{}},
			{State: ghapi.Ptr("CHANGES_REQUESTED"), SubmittedAt: &ghapi.Timestamp{}},
		}
		json.NewEncoder(w).Encode(reviews)
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/def456/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.CombinedStatus{Statuses: []*ghapi.RepoStatus{}})
	})

	mux.HandleFunc("GET /repos/owner/repo/commits/def456/check-runs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ghapi.ListCheckRunsResults{Total: ghapi.Ptr(0), CheckRuns: []*ghapi.CheckRun{}})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghapi.NewClient(nil).WithAuthToken("test")
	baseURL, _ := url.Parse(ts.URL + "/")
	client.BaseURL = baseURL

	s := &GitHubSource{client: client}
	m := map[string]any{}

	subjectURL, _ := url.Parse(ts.URL + "/repos/owner/repo/pulls/10")
	err := s.enrichPullRequest(context.Background(), subjectURL, "owner", "repo", m)
	if err != nil {
		t.Fatalf("enrichPullRequest: %v", err)
	}

	if m["review_decision"] != "CHANGES_REQUESTED" {
		t.Errorf("review_decision = %v, want CHANGES_REQUESTED", m["review_decision"])
	}
	if m["draft"] != true {
		t.Errorf("draft = %v, want true", m["draft"])
	}
}
