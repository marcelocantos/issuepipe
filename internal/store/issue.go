// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package store

// Issue is one GitHub issue row keyed by stable node ids.
type Issue struct {
	IssueNodeID string
	RepoID      int64
	RepoNodeID  string
	Number      int64
	Title       string
	Body        string
	State       string
	LabelsJSON  string
	AuthorLogin string
	HTMLURL     string
	UpdatedAt   string
	RawJSON     string
}
