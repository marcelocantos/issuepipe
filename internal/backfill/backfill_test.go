// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package backfill_test

import (
	"context"
	"testing"

	"github.com/marcelocantos/issuepipe/internal/backfill"
	"github.com/marcelocantos/issuepipe/internal/store"
)

type fakeLister struct {
	byRepo map[int64][]store.Issue
}

func (f *fakeLister) ListIssues(_ context.Context, repoID int64) ([]store.Issue, error) {
	return f.byRepo[repoID], nil
}

func TestColdStartBackfillReconstructs(t *testing.T) {
	dir := t.TempDir()
	cat, err := store.OpenCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Seed an empty on-disk repo DB (volume "exists" after restart).
	if _, err := cat.Get(555); err != nil {
		t.Fatal(err)
	}
	_ = cat.Close()

	cat2, err := store.OpenCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat2.Close()

	lister := &fakeLister{byRepo: map[int64][]store.Issue{
		555: {{
			IssueNodeID: "I_bf1",
			RepoID:      555,
			RepoNodeID:  "R_555",
			Number:      3,
			Title:       "from backfill",
			State:       "open",
			LabelsJSON:  "[]",
			UpdatedAt:   "2026-07-01T00:00:00Z",
			RawJSON:     "{}",
		}},
	}}
	runner := &backfill.Runner{Catalog: cat2, Lister: lister}
	if err := runner.RunForDiscovered(context.Background()); err != nil {
		t.Fatal(err)
	}
	repo, err := cat2.Get(555)
	if err != nil {
		t.Fatal(err)
	}
	n, err := repo.CountIssues()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	issue, ok, err := repo.GetIssue("I_bf1")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if issue.Title != "from backfill" {
		t.Fatalf("title = %q", issue.Title)
	}
}
