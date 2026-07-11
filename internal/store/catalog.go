// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/marcelocantos/sqlpipe/go/sqlpipe"
)

// Catalog holds one sqlpipe Master database per GitHub repository id.
// Per-repo partitioning is required so authorization (T32) can gate
// convergence at table/DB ownership granularity without leaking
// existence of unauthorized rows via bucket-hash diffs.
type Catalog struct {
	mu   sync.Mutex
	root string
	// repos maps GitHub repository numeric id → open store.
	repos map[int64]*RepoDB
}

// RepoDB is one repository's SQLite file plus its sqlpipe Master.
type RepoDB struct {
	RepoID int64
	Path   string
	DB     *sqlpipe.Database
	Master *sqlpipe.Master
}

// OpenCatalog creates or opens a catalog rooted at dir.
func OpenCatalog(dir string) (*Catalog, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &Catalog{
		root:  dir,
		repos: make(map[int64]*RepoDB),
	}, nil
}

// Close closes every open repo database.
func (c *Catalog) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var first error
	for id, r := range c.repos {
		if err := r.close(); err != nil && first == nil {
			first = err
		}
		delete(c.repos, id)
	}
	return first
}

// Get returns the open RepoDB for repoID, opening it on first use.
func (c *Catalog) Get(repoID int64) (*RepoDB, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r, ok := c.repos[repoID]; ok {
		return r, nil
	}
	path := filepath.Join(c.root, fmt.Sprintf("repo_%d.db", repoID))
	// Open without schemaDDL: sqlift migrate would treat sqlpipe meta
	// tables as "undesired" on subsequent opens.
	db, err := sqlpipe.OpenDatabase(path)
	if err != nil {
		return nil, fmt.Errorf("open repo %d db: %w", repoID, err)
	}
	if err := db.Exec(IssuesDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensure issues schema for repo %d: %w", repoID, err)
	}
	master, err := sqlpipe.NewMaster(db, sqlpipe.MasterConfig{})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("new master for repo %d: %w", repoID, err)
	}
	r := &RepoDB{RepoID: repoID, Path: path, DB: db, Master: master}
	c.repos[repoID] = r
	return r, nil
}

// ListOpen returns a snapshot of currently open repo IDs.
func (c *Catalog) ListOpen() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]int64, 0, len(c.repos))
	for id := range c.repos {
		out = append(out, id)
	}
	return out
}

// DiscoverRepoIDs lists on-disk repo_*.db files under the catalog root
// (used by cold-start backfill when the process restarts with an empty
// in-memory map but a surviving volume).
func (c *Catalog) DiscoverRepoIDs() ([]int64, error) {
	entries, err := os.ReadDir(c.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(e.Name(), "repo_%d.db", &id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *RepoDB) close() error {
	var first error
	if r.Master != nil {
		if err := r.Master.Close(); err != nil && first == nil {
			first = err
		}
		r.Master = nil
	}
	if r.DB != nil {
		if err := r.DB.Close(); err != nil && first == nil {
			first = err
		}
		r.DB = nil
	}
	return first
}

// UpsertIssue inserts or replaces an issue row and flushes the Master
// changeset queue so Replicas can converge the change.
//
// Upsert is idempotent: redelivery of the same event yields one row.
func (r *RepoDB) UpsertIssue(issue Issue) error {
	// Parameterized write on the shared Database, then Master.Flush so
	// the session extension records the change (sqlpipe Go tests use
	// this pattern; Master.Exec has no bind-arg surface).
	const q = `
INSERT INTO issues (
  issue_node_id, repo_id, repo_node_id, number, title, body, state,
  labels_json, author_login, html_url, updated_at, raw_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(issue_node_id) DO UPDATE SET
  repo_id = excluded.repo_id,
  repo_node_id = excluded.repo_node_id,
  number = excluded.number,
  title = excluded.title,
  body = excluded.body,
  state = excluded.state,
  labels_json = excluded.labels_json,
  author_login = excluded.author_login,
  html_url = excluded.html_url,
  updated_at = excluded.updated_at,
  raw_json = excluded.raw_json
`
	if err := r.DB.Exec(q,
		issue.IssueNodeID,
		issue.RepoID,
		issue.RepoNodeID,
		issue.Number,
		issue.Title,
		issue.Body,
		issue.State,
		issue.LabelsJSON,
		issue.AuthorLogin,
		issue.HTMLURL,
		issue.UpdatedAt,
		issue.RawJSON,
	); err != nil {
		return fmt.Errorf("upsert issue: %w", err)
	}
	if _, err := r.Master.Flush(); err != nil {
		return fmt.Errorf("master flush: %w", err)
	}
	return nil
}

// CountIssues returns the number of rows in the issues table.
func (r *RepoDB) CountIssues() (int64, error) {
	res, err := r.DB.Query("SELECT COUNT(*) FROM issues")
	if err != nil {
		return 0, err
	}
	if len(res.Rows) == 0 {
		return 0, fmt.Errorf("empty count result")
	}
	switch v := res.Rows[0][0].(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected count type %T", res.Rows[0][0])
	}
}

// GetIssue returns one issue by node id, or false if missing.
func (r *RepoDB) GetIssue(issueNodeID string) (Issue, bool, error) {
	res, err := r.DB.Query(
		`SELECT issue_node_id, repo_id, repo_node_id, number, title, body, state,
		        labels_json, author_login, html_url, updated_at, raw_json
		 FROM issues WHERE issue_node_id = ?`,
		issueNodeID,
	)
	if err != nil {
		return Issue{}, false, err
	}
	if len(res.Rows) == 0 {
		return Issue{}, false, nil
	}
	row := res.Rows[0]
	return Issue{
		IssueNodeID: asString(row[0]),
		RepoID:      asInt64(row[1]),
		RepoNodeID:  asString(row[2]),
		Number:      asInt64(row[3]),
		Title:       asString(row[4]),
		Body:        asString(row[5]),
		State:       asString(row[6]),
		LabelsJSON:  asString(row[7]),
		AuthorLogin: asString(row[8]),
		HTMLURL:     asString(row[9]),
		UpdatedAt:   asString(row[10]),
		RawJSON:     asString(row[11]),
	}, true, nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case float64:
		return int64(t)
	case int:
		return int64(t)
	default:
		return 0
	}
}
