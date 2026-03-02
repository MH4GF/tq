package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"

	ghapi "github.com/google/go-github/v71/github"
	"github.com/shurcooL/githubv4"

	"github.com/MH4GF/tq/source"
)

// Fetch retrieves all GitHub notifications from Inbox, enriches them,
// marks skip targets (merged/closed/unknown) as Done, and returns active ones.
func (s *GitHubSource) Fetch(ctx context.Context) ([]source.Notification, error) {
	s.results = nil

	sem := make(chan struct{}, 10)
	page := 1
	for {
		notifications, resp, err := s.client.Activity.ListNotifications(ctx, &ghapi.NotificationListOptions{
			ListOptions: ghapi.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("list notifications: %w", err)
		}

		var wg sync.WaitGroup
		for _, n := range notifications {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				s.processNotification(ctx, n)
			}()
		}
		wg.Wait()

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	slog.Info("fetched notifications", "count", len(s.results))

	var out []source.Notification
	for _, r := range s.results {
		if r.skip {
			if err := s.markThreadDone(ctx, r.threadID); err != nil {
				slog.Error("failed to mark skipped thread done", "id", r.threadID, "error", err)
			}
			continue
		}
		out = append(out, r.notification)
	}
	return out, nil
}

// MarkProcessed marks a GitHub notification thread as Done (removes from Inbox).
func (s *GitHubSource) MarkProcessed(ctx context.Context, n source.Notification) error {
	threadID, ok := n.Metadata["id"].(string)
	if !ok {
		return fmt.Errorf("missing thread id in metadata")
	}
	return s.markThreadDone(ctx, threadID)
}

func (s *GitHubSource) markThreadDone(ctx context.Context, threadID string) error {
	id, err := strconv.ParseInt(threadID, 10, 64)
	if err != nil {
		return fmt.Errorf("parse thread id %q: %w", threadID, err)
	}
	if _, err := s.client.Activity.MarkThreadDone(ctx, id); err != nil {
		return fmt.Errorf("mark thread done: %w", err)
	}
	return nil
}

func (s *GitHubSource) processNotification(ctx context.Context, n *ghapi.Notification) {
	threadID := n.GetID()
	subjectURL := n.GetSubject().GetURL()
	if subjectURL == "" {
		slog.Warn("notification has no subject URL, skipping", "id", threadID)
		s.addResult(enrichedNotification{threadID: threadID, skip: true})
		return
	}

	u, err := url.Parse(subjectURL)
	if err != nil {
		slog.Error("parse subject URL", "id", threadID, "error", err)
		s.addResult(enrichedNotification{threadID: threadID, skip: true})
		return
	}

	owner := n.GetRepository().GetOwner().GetLogin()
	repo := n.GetRepository().GetName()
	subjectType := n.GetSubject().GetType()
	reason := n.GetReason()
	title := n.GetSubject().GetTitle()

	m := map[string]any{
		"id":           threadID,
		"reason":       reason,
		"subject_type": subjectType,
		"repo":         owner + "/" + repo,
		"title":        title,
	}

	switch subjectType {
	case "Issue":
		if err := s.enrichIssue(ctx, u, owner, repo, m); err != nil {
			slog.Error("enrich issue failed", "id", threadID, "error", err)
			s.addResult(enrichedNotification{threadID: threadID, skip: true})
			return
		}
	case "PullRequest":
		if err := s.enrichPullRequest(ctx, u, owner, repo, m); err != nil {
			slog.Error("enrich PR failed", "id", threadID, "error", err)
			s.addResult(enrichedNotification{threadID: threadID, skip: true})
			return
		}
	case "Discussion":
		if err := s.enrichDiscussion(ctx, u, owner, repo, m); err != nil {
			slog.Error("enrich discussion failed", "id", threadID, "error", err)
			s.addResult(enrichedNotification{threadID: threadID, skip: true})
			return
		}
	case "Release":
		if err := s.enrichRelease(ctx, u, owner, repo, m); err != nil {
			slog.Error("enrich release failed", "id", threadID, "error", err)
			s.addResult(enrichedNotification{threadID: threadID, skip: true})
			return
		}
	default:
		slog.Warn("unknown subject type, skipping", "type", subjectType)
		s.addResult(enrichedNotification{threadID: threadID, skip: true})
		return
	}

	if isMergedOrClosed(m) {
		s.addResult(enrichedNotification{threadID: threadID, skip: true})
		return
	}

	msg := buildMessage(m)
	notification := source.Notification{
		Source:   "gh-notification",
		Message:  msg,
		Metadata: m,
	}
	s.addResult(enrichedNotification{
		notification: notification,
		threadID:     threadID,
	})
}

func (s *GitHubSource) enrichIssue(ctx context.Context, u *url.URL, owner, repo string, m map[string]any) error {
	number, err := strconv.Atoi(path.Base(u.Path))
	if err != nil {
		return fmt.Errorf("parse issue number: %w", err)
	}

	issue, _, err := s.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		var errResp *ghapi.ErrorResponse
		if errors.As(err, &errResp) && errResp.Response.StatusCode == http.StatusNotFound {
			slog.Warn("issue not found, skipping", "owner", owner, "repo", repo, "number", number)
			return nil
		}
		return fmt.Errorf("get issue: %w", err)
	}

	m["url"] = issue.GetHTMLURL()
	m["state"] = issue.GetState()
	m["author"] = issue.GetUser().GetLogin()
	m["labels"] = labelNames(issue.Labels)
	m["assignees"] = userLogins(issue.Assignees)
	return nil
}

func (s *GitHubSource) enrichDiscussion(ctx context.Context, u *url.URL, owner, repo string, m map[string]any) error {
	number, err := strconv.Atoi(path.Base(u.Path))
	if err != nil {
		return fmt.Errorf("parse discussion number: %w", err)
	}

	var q discussionQuery
	variables := map[string]any{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(int32(number)), //nolint:gosec
	}
	if err := s.v4Client.Query(ctx, &q, variables); err != nil {
		slog.Warn("discussion fetch error, skipping", "owner", owner, "repo", repo, "number", number, "error", err)
		return nil
	}

	d := q.Repository.Discussion
	m["url"] = d.URL
	m["state"] = "open"
	if d.Closed {
		m["state"] = "closed"
	}
	m["author"] = d.Author.Login
	labels := make([]string, 0, len(d.Labels.Nodes))
	for _, l := range d.Labels.Nodes {
		labels = append(labels, l.Name)
	}
	m["labels"] = labels
	return nil
}

func (s *GitHubSource) enrichRelease(ctx context.Context, u *url.URL, owner, repo string, m map[string]any) error {
	id, err := strconv.Atoi(path.Base(u.Path))
	if err != nil {
		return fmt.Errorf("parse release ID: %w", err)
	}

	r, _, err := s.client.Repositories.GetRelease(ctx, owner, repo, int64(id))
	if err != nil {
		var errResp *ghapi.ErrorResponse
		if errors.As(err, &errResp) && errResp.Response.StatusCode == http.StatusNotFound {
			slog.Warn("release not found, skipping", "owner", owner, "repo", repo, "id", id)
			return nil
		}
		return fmt.Errorf("get release: %w", err)
	}

	m["url"] = r.GetHTMLURL()
	m["state"] = "published"
	m["author"] = r.GetAuthor().GetLogin()
	return nil
}

func isMergedOrClosed(m map[string]any) bool {
	if merged, ok := m["merged"].(bool); ok && merged {
		return true
	}
	if state, ok := m["state"].(string); ok && state == "closed" {
		return true
	}
	return false
}

func buildMessage(m map[string]any) string {
	reason := m["reason"].(string)
	title := m["title"].(string)
	repo := m["repo"].(string)
	subjectType := m["subject_type"].(string)
	urlStr, _ := m["url"].(string)

	var parts []string
	parts = append(parts, fmt.Sprintf("[GitHub通知] %s: %s (%s)", reasonLabel(reason), title, repo))

	if subjectType == "PullRequest" && urlStr != "" {
		parts = append(parts, urlStr)
		var status []string
		if approved, ok := m["approved"].(bool); ok {
			status = append(status, fmt.Sprintf("approved=%v", approved))
		}
		if ciPassed, ok := m["ci_passed"].(bool); ok {
			status = append(status, fmt.Sprintf("ci_passed=%v", ciPassed))
		}
		if ciFailed, ok := m["ci_failed"].(bool); ok && ciFailed {
			status = append(status, "ci_failed=true")
		}
		if mergeable, ok := m["mergeable"].(string); ok {
			status = append(status, fmt.Sprintf("mergeable=%s", mergeable))
		}
		if len(status) > 0 {
			parts = append(parts, strings.Join(status, ", "))
		}
	} else if urlStr != "" {
		parts = append(parts, urlStr)
	}

	return strings.Join(parts, " | ")
}

func reasonLabel(reason string) string {
	switch reason {
	case "review_requested":
		return "レビュー依頼"
	case "comment":
		return "コメント"
	case "mention":
		return "メンション"
	case "ci_activity":
		return "CI"
	case "state_change":
		return "状態変更"
	case "assign":
		return "アサイン"
	case "author":
		return "作成者通知"
	case "subscribed":
		return "購読"
	default:
		return reason
	}
}

func labelNames(labels []*ghapi.Label) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.GetName())
	}
	return out
}

func userLogins(users []*ghapi.User) []string {
	out := make([]string, 0, len(users))
	for _, u := range users {
		out = append(out, u.GetLogin())
	}
	return out
}
