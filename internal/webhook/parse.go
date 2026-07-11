// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"encoding/json"
	"fmt"

	"github.com/marcelocantos/issuepipe/internal/store"
)

// issuesEvent is the subset of a GitHub issues webhook payload we need.
type issuesEvent struct {
	Action string `json:"action"`
	Issue  struct {
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
	} `json:"issue"`
	Repository struct {
		ID     int64  `json:"id"`
		NodeID string `json:"node_id"`
	} `json:"repository"`
}

// ParseIssuesEvent maps a GitHub issues webhook JSON body to a store.Issue.
// Returns (nil, nil) for events that are not issues-shaped or lack stable ids.
func ParseIssuesEvent(body []byte) (*store.Issue, error) {
	var ev issuesEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, fmt.Errorf("parse issues event: %w", err)
	}
	if ev.Issue.NodeID == "" || ev.Repository.ID == 0 {
		return nil, nil
	}
	labels, err := json.Marshal(labelNames(ev.Issue.Labels))
	if err != nil {
		return nil, err
	}
	bodyText := ev.Issue.Body
	if bodyText == "" {
		// GitHub may send null body; keep empty string in SQLite.
		bodyText = ""
	}
	return &store.Issue{
		IssueNodeID: ev.Issue.NodeID,
		RepoID:      ev.Repository.ID,
		RepoNodeID:  ev.Repository.NodeID,
		Number:      ev.Issue.Number,
		Title:       ev.Issue.Title,
		Body:        bodyText,
		State:       ev.Issue.State,
		LabelsJSON:  string(labels),
		AuthorLogin: ev.Issue.User.Login,
		HTMLURL:     ev.Issue.HTMLURL,
		UpdatedAt:   ev.Issue.UpdatedAt,
		RawJSON:     string(body),
	}, nil
}

func labelNames(labels []struct {
	Name string `json:"name"`
}) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.Name)
	}
	return out
}
