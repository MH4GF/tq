package github

import (
	"fmt"
	"sync"

	ghapi "github.com/google/go-github/v71/github"
	"github.com/k1LoW/go-github-client/v71/factory"
	"github.com/shurcooL/githubv4"

	"github.com/MH4GF/tq/source"
)

// discussionQuery is the GraphQL query for fetching a discussion.
type discussionQuery struct {
	Repository struct {
		Discussion struct {
			Title      string
			URL        string
			Closed     bool
			Number     int
			IsAnswered bool
			Author     struct {
				Login string
			}
			Labels struct {
				Nodes []struct {
					Name string
				}
			} `graphql:"labels(first: 100)"`
		} `graphql:"discussion(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// GitHubSource fetches unread GitHub notifications and enriches them.
type GitHubSource struct {
	client   *ghapi.Client
	v4Client *githubv4.Client
	mu       sync.Mutex
	results  []enrichedNotification
}

type enrichedNotification struct {
	notification source.Notification
	threadID     string
	skip         bool
}

// NewGitHubSource creates a GitHubSource using gh CLI authentication.
func NewGitHubSource() (*GitHubSource, error) {
	client, err := factory.NewGithubClient()
	if err != nil {
		return nil, fmt.Errorf("create github client: %w", err)
	}
	v4Client := githubv4.NewClient(client.Client())

	return &GitHubSource{
		client:   client,
		v4Client: v4Client,
	}, nil
}

// NewGitHubSourceWithClients creates a GitHubSource with provided clients (for testing).
func NewGitHubSourceWithClients(client *ghapi.Client, v4Client *githubv4.Client) *GitHubSource {
	return &GitHubSource{
		client:   client,
		v4Client: v4Client,
	}
}

func (s *GitHubSource) Name() string { return "gh-notification" }

func (s *GitHubSource) addResult(r enrichedNotification) {
	s.mu.Lock()
	s.results = append(s.results, r)
	s.mu.Unlock()
}
