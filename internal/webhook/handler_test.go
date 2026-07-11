// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package webhook_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/marcelocantos/issuepipe/internal/store"
	"github.com/marcelocantos/issuepipe/internal/webhook"
)

const sampleIssuesEvent = `{
  "action": "opened",
  "issue": {
    "node_id": "I_kwDOABCD123",
    "number": 42,
    "title": "Fix the thing",
    "body": "details",
    "state": "open",
    "html_url": "https://github.com/acme/widget/issues/42",
    "updated_at": "2026-07-11T00:00:00Z",
    "user": {"login": "alice"},
    "labels": [{"name": "bug"}]
  },
  "repository": {
    "id": 99112233,
    "node_id": "R_kgDOABCDxyz"
  }
}`

func TestHandlerRejectsInvalidSignature(t *testing.T) {
	cat := openCatalog(t)
	h := &webhook.Handler{Secret: "sekrit", Catalog: cat}
	body := []byte(sampleIssuesEvent)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	req.Header.Set("X-GitHub-Event", "issues")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestHandlerAcceptsValidIssuesEvent(t *testing.T) {
	cat := openCatalog(t)
	secret := "sekrit"
	h := &webhook.Handler{Secret: secret, Catalog: cat}
	body := []byte(sampleIssuesEvent)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", webhook.SignBody(secret, body))
	req.Header.Set("X-GitHub-Event", "issues")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}

	repo, err := cat.Get(99112233)
	if err != nil {
		t.Fatal(err)
	}
	issue, ok, err := repo.GetIssue("I_kwDOABCD123")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("issue not found")
	}
	if issue.Number != 42 || issue.Title != "Fix the thing" {
		t.Fatalf("unexpected issue: %+v", issue)
	}
	if issue.RepoID != 99112233 {
		t.Fatalf("repo id = %d", issue.RepoID)
	}
}

func TestHandlerIdempotentRedelivery(t *testing.T) {
	cat := openCatalog(t)
	secret := "sekrit"
	h := &webhook.Handler{Secret: secret, Catalog: cat}
	body := []byte(sampleIssuesEvent)
	sig := webhook.SignBody(secret, body)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", sig)
		req.Header.Set("X-GitHub-Event", "issues")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("delivery %d: status %d", i, rr.Code)
		}
	}

	repo, err := cat.Get(99112233)
	if err != nil {
		t.Fatal(err)
	}
	n, err := repo.CountIssues()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1 after redelivery", n)
	}
}

func TestHandlerPing(t *testing.T) {
	cat := openCatalog(t)
	secret := "sekrit"
	h := &webhook.Handler{Secret: secret, Catalog: cat}
	body := []byte(`{"zen":"Non-blocking is better than blocking."}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", webhook.SignBody(secret, body))
	req.Header.Set("X-GitHub-Event", "ping")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}

func openCatalog(t *testing.T) *store.Catalog {
	t.Helper()
	dir := t.TempDir()
	cat, err := store.OpenCatalog(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cat.Close() })
	return cat
}
