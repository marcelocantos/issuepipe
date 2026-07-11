// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package backfill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/marcelocantos/issuepipe/internal/store"
)

// IssueLister fetches open (and optionally closed) issues for a repo.
// Tests inject a fake; production uses GitHubAppClient.
type IssueLister interface {
	ListIssues(ctx context.Context, repoID int64) ([]store.Issue, error)
}

// Runner reconstructs per-repo tables from GitHub after a cold start
// (or lost Master volume). This is the single place a reconcile-poll lives;
// steady-state delivery is webhook-driven.
type Runner struct {
	Catalog *store.Catalog
	Lister  IssueLister
}

// RunForDiscoveredRepos every on-disk repo and every currently open
// repo through the lister and upserts results.
func (r *Runner) RunForDiscovered(ctx context.Context) error {
	ids, err := r.Catalog.DiscoverRepoIDs()
	if err != nil {
		return err
	}
	// Also include in-memory opens that may not yet be on disk naming
	// convention (should not happen, but keeps cold-start + live union).
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	for _, id := range r.Catalog.ListOpen() {
		if _, ok := seen[id]; !ok {
			ids = append(ids, id)
			seen[id] = struct{}{}
		}
	}
	for _, id := range ids {
		if err := r.RunRepo(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// RunRepo backfills one repository id.
func (r *Runner) RunRepo(ctx context.Context, repoID int64) error {
	if r.Lister == nil {
		return fmt.Errorf("backfill: no IssueLister configured")
	}
	issues, err := r.Lister.ListIssues(ctx, repoID)
	if err != nil {
		return fmt.Errorf("list issues for repo %d: %w", repoID, err)
	}
	repo, err := r.Catalog.Get(repoID)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		if issue.RepoID == 0 {
			issue.RepoID = repoID
		}
		if err := repo.UpsertIssue(issue); err != nil {
			return fmt.Errorf("upsert issue %s: %w", issue.IssueNodeID, err)
		}
	}
	log.Printf("backfill: repo %d — %d issues upserted", repoID, len(issues))
	return nil
}

// GitHubAppClient lists issues for a repository via the GitHub REST API
// using an installation access token supplier.
type GitHubAppClient struct {
	// BaseURL defaults to https://api.github.com.
	BaseURL string
	// Token returns a bearer token for the installation that owns repoID.
	Token func(ctx context.Context, repoID int64) (string, error)
	// HTTP is optional; defaults to http.DefaultClient.
	HTTP *http.Client
	// Now is optional clock for tests.
	Now func() time.Time
}

// ListIssues implements IssueLister by paginating GET /repositories/{id}/issues.
func (c *GitHubAppClient) ListIssues(ctx context.Context, repoID int64) ([]store.Issue, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	token, err := c.Token(ctx, repoID)
	if err != nil {
		return nil, err
	}

	var out []store.Issue
	page := 1
	for {
		u, err := url.Parse(fmt.Sprintf("%s/repositories/%d/issues", base, repoID))
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("state", "all")
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		var payload []ghIssue
		decErr := json.NewDecoder(resp.Body).Decode(&payload)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("github list issues repo %d: status %d", repoID, resp.StatusCode)
		}
		if decErr != nil {
			return nil, decErr
		}
		if len(payload) == 0 {
			break
		}
		for _, g := range payload {
			// Skip PRs — GitHub includes pull requests in /issues.
			if g.PullRequest != nil {
				continue
			}
			labels, _ := json.Marshal(g.labelNames())
			raw, _ := json.Marshal(g)
			out = append(out, store.Issue{
				IssueNodeID: g.NodeID,
				RepoID:      repoID,
				RepoNodeID:  g.RepositoryNodeID, // often empty on list; ok
				Number:      g.Number,
				Title:       g.Title,
				Body:        g.Body,
				State:       g.State,
				LabelsJSON:  string(labels),
				AuthorLogin: g.User.Login,
				HTMLURL:     g.HTMLURL,
				UpdatedAt:   g.UpdatedAt,
				RawJSON:     string(raw),
			})
		}
		if len(payload) < 100 {
			break
		}
		page++
	}
	return out, nil
}

type ghIssue struct {
	NodeID    string `json:"node_id"`
	Number    int64  `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest      *json.RawMessage `json:"pull_request"`
	RepositoryNodeID string           `json:"-"`
}

func (g ghIssue) labelNames() []string {
	out := make([]string, 0, len(g.Labels))
	for _, l := range g.Labels {
		out = append(out, l.Name)
	}
	return out
}
