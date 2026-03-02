package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"

	ghapi "github.com/google/go-github/v71/github"
)

func (s *GitHubSource) enrichPullRequest(ctx context.Context, u *url.URL, owner, repo string, m map[string]any) error {
	number, err := strconv.Atoi(path.Base(u.Path))
	if err != nil {
		return fmt.Errorf("parse PR number: %w", err)
	}

	pr, _, err := s.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		var errResp *ghapi.ErrorResponse
		if errors.As(err, &errResp) && errResp.Response.StatusCode == http.StatusNotFound {
			slog.Warn("PR not found, skipping", "owner", owner, "repo", repo, "number", number)
			return nil
		}
		return fmt.Errorf("get PR: %w", err)
	}

	m["url"] = pr.GetHTMLURL()
	m["state"] = pr.GetState()
	m["author"] = pr.GetUser().GetLogin()
	m["merged"] = pr.GetMerged()
	m["draft"] = pr.GetDraft()
	m["mergeable"] = pr.GetMergeableState()
	m["labels"] = labelNames(pr.Labels)
	m["assignees"] = userLogins(pr.Assignees)

	// Reviews
	reviews, _, err := s.client.PullRequests.ListReviews(ctx, owner, repo, number, &ghapi.ListOptions{})
	if err != nil {
		return fmt.Errorf("list reviews: %w", err)
	}

	slices.SortFunc(reviews, func(a, b *ghapi.PullRequestReview) int {
		return a.GetSubmittedAt().Compare(b.GetSubmittedAt().Time)
	})

	approved := false
	reviewDecision := "REVIEW_REQUIRED"
	for _, review := range reviews {
		switch review.GetState() {
		case "APPROVED":
			approved = true
			reviewDecision = "APPROVED"
		case "CHANGES_REQUESTED":
			reviewDecision = "CHANGES_REQUESTED"
		}
	}
	m["approved"] = approved
	m["review_decision"] = reviewDecision

	// CI status
	commitSHA := pr.GetHead().GetSHA()
	ciPassed, ciFailed := s.checkCIStatus(ctx, owner, repo, commitSHA)
	m["ci_passed"] = ciPassed
	m["ci_failed"] = ciFailed

	return nil
}

func (s *GitHubSource) checkCIStatus(ctx context.Context, owner, repo, commitSHA string) (passed, failed bool) {
	combinedStatus, _, err := s.client.Repositories.GetCombinedStatus(ctx, owner, repo, commitSHA, &ghapi.ListOptions{})
	if err != nil {
		slog.Error("failed to get combined status", "error", err)
		return false, false
	}

	statusPassed := true
	statusFailed := false
	for _, status := range combinedStatus.Statuses {
		switch status.GetState() {
		case "success":
			continue
		case "failure", "startup_failure":
			statusPassed = false
			statusFailed = true
		default:
			statusPassed = false
		}
	}

	checkRuns, _, err := s.client.Checks.ListCheckRunsForRef(ctx, owner, repo, commitSHA, &ghapi.ListCheckRunsOptions{})
	if err != nil {
		slog.Error("failed to list check runs", "error", err)
		return statusPassed, statusFailed
	}

	checksPassed := true
	checksFailed := false
	for _, cr := range checkRuns.CheckRuns {
		switch {
		case cr.GetStatus() == "completed" && slices.Contains([]string{"neutral", "skipped", "success"}, cr.GetConclusion()):
			continue
		case slices.Contains([]string{"failure", "startup_failure"}, cr.GetStatus()) || slices.Contains([]string{"canceled", "failure", "stale", "timed_out"}, cr.GetConclusion()):
			checksPassed = false
			checksFailed = true
		case slices.Contains([]string{"expected", "in_progress", "pending", "queued", "requested", "waiting"}, cr.GetStatus()):
			checksPassed = false
		}
	}

	return statusPassed && checksPassed, statusFailed || checksFailed
}
