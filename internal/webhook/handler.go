// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"io"
	"log"
	"net/http"

	"github.com/marcelocantos/issuepipe/internal/store"
)

// Handler serves GitHub App issue webhooks into a store.Catalog.
type Handler struct {
	Secret  string
	Catalog *store.Catalog
	// MaxBody is the max request body size (default 1 MiB).
	MaxBody int64
}

// ServeHTTP implements http.Handler for POST /webhook/github.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	max := h.MaxBody
	if max <= 0 {
		max = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, max+1))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > max {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	sig := r.Header.Get("X-Hub-Signature-256")
	if err := VerifySignature(h.Secret, body, sig); err != nil {
		log.Printf("webhook: reject signature: %v", err)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	// ping is delivery-check only.
	if event == "ping" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"event":"ping"}`))
		return
	}
	if event != "" && event != "issues" {
		// Accept but ignore non-issues events so GitHub does not retry.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"ignored":true}`))
		return
	}

	issue, err := ParseIssuesEvent(body)
	if err != nil {
		log.Printf("webhook: parse: %v", err)
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	if issue == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"ignored":true}`))
		return
	}

	repo, err := h.Catalog.Get(issue.RepoID)
	if err != nil {
		log.Printf("webhook: open repo %d: %v", issue.RepoID, err)
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}
	if err := repo.UpsertIssue(*issue); err != nil {
		log.Printf("webhook: upsert: %v", err)
		http.Error(w, "upsert error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}
