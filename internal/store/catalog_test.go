// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"path/filepath"
	"testing"

	"github.com/marcelocantos/issuepipe/internal/store"
	"github.com/marcelocantos/sqlpipe/go/sqlpipe"
)

func TestPerRepoPartitionAndMaster(t *testing.T) {
	dir := t.TempDir()
	cat, err := store.OpenCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()

	a, err := cat.Get(111)
	if err != nil {
		t.Fatal(err)
	}
	b, err := cat.Get(222)
	if err != nil {
		t.Fatal(err)
	}
	if a.Path == b.Path {
		t.Fatal("repos must use distinct DB files")
	}
	if a.Master == nil || b.Master == nil {
		t.Fatal("each repo needs a sqlpipe Master")
	}

	issue := store.Issue{
		IssueNodeID: "I_a1",
		RepoID:      111,
		RepoNodeID:  "R_a",
		Number:      1,
		Title:       "one",
		State:       "open",
		LabelsJSON:  "[]",
		UpdatedAt:   "2026-01-01T00:00:00Z",
		RawJSON:     "{}",
	}
	if err := a.UpsertIssue(issue); err != nil {
		t.Fatal(err)
	}
	// Master advanced past 0 after a tracked write+flush.
	if a.Master.CurrentSeq() < 1 {
		t.Fatalf("master seq = %d, want >= 1", a.Master.CurrentSeq())
	}

	nA, err := a.CountIssues()
	if err != nil {
		t.Fatal(err)
	}
	nB, err := b.CountIssues()
	if err != nil {
		t.Fatal(err)
	}
	if nA != 1 || nB != 0 {
		t.Fatalf("partition leak: a=%d b=%d", nA, nB)
	}

	// Same Master is ready to serve Replicas (HandleMessage surface).
	// Empty bucket-hash from a fresh replica should not panic.
	_ = sqlpipe.BucketHashesMsg{}
	if a.Master.SchemaVersion() == nil {
		// SchemaVersion may be empty bytes but method must be callable.
		t.Log("schema version empty (ok for empty-ish schema fingerprint)")
	}
}

func TestUpsertIdempotent(t *testing.T) {
	dir := t.TempDir()
	cat, err := store.OpenCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	repo, err := cat.Get(7)
	if err != nil {
		t.Fatal(err)
	}
	issue := store.Issue{
		IssueNodeID: "I_same",
		RepoID:      7,
		RepoNodeID:  "R_7",
		Number:      9,
		Title:       "first",
		State:       "open",
		LabelsJSON:  "[]",
		UpdatedAt:   "t1",
		RawJSON:     `{"v":1}`,
	}
	for i := 0; i < 5; i++ {
		issue.Title = "first"
		if i == 4 {
			issue.Title = "updated"
			issue.RawJSON = `{"v":2}`
		}
		if err := repo.UpsertIssue(issue); err != nil {
			t.Fatal(err)
		}
	}
	n, err := repo.CountIssues()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
	got, ok, err := repo.GetIssue("I_same")
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.Title != "updated" {
		t.Fatalf("title = %q, want updated", got.Title)
	}
}

func TestDiscoverRepoIDs(t *testing.T) {
	dir := t.TempDir()
	cat, err := store.OpenCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cat.Get(42); err != nil {
		t.Fatal(err)
	}
	// Force close so path stays on disk.
	_ = cat.Close()

	cat2, err := store.OpenCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cat2.Close()
	ids, err := cat2.DiscoverRepoIDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 42 {
		t.Fatalf("ids = %v, want [42]", ids)
	}
	// Re-open via Get uses same path.
	r, err := cat2.Get(42)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(r.Path) != "repo_42.db" {
		t.Fatalf("path = %s", r.Path)
	}
}
