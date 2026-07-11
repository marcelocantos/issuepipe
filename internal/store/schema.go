// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package store

// IssuesDDL creates the per-repo issues table when missing.
//
// sqlpipe Master requires an INTEGER PRIMARY KEY rowid alias on every
// replicated table. Business identity stays on issue_node_id (unique):
// stable GitHub node ids, never owner/name strings.
//
// We use CREATE TABLE IF NOT EXISTS (not sqlift open-time migrate) so
// reopening an existing Master DB does not try to "migrate away"
// sqlpipe's own _sqlpipe_meta tables.
const IssuesDDL = `
CREATE TABLE IF NOT EXISTS issues (
  id INTEGER PRIMARY KEY,
  issue_node_id TEXT NOT NULL UNIQUE,
  repo_id INTEGER NOT NULL,
  repo_node_id TEXT NOT NULL,
  number INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT 'open',
  labels_json TEXT NOT NULL DEFAULT '[]',
  author_login TEXT NOT NULL DEFAULT '',
  html_url TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  raw_json TEXT NOT NULL DEFAULT ''
);
`
