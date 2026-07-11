// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

// Command issuepipe is a Fly-hosted sqlpipe Master that ingests GitHub App
// issue webhooks into per-repo SQLite tables (bullseye 🎯T31).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marcelocantos/issuepipe/internal/backfill"
	"github.com/marcelocantos/issuepipe/internal/config"
	"github.com/marcelocantos/issuepipe/internal/store"
	"github.com/marcelocantos/issuepipe/internal/webhook"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	showHelp := flag.Bool("help", false, "print help and exit")
	showAgent := flag.Bool("help-agent", false, "print agent guide and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("issuepipe 0.1.0")
		return
	}
	if *showHelp {
		fmt.Print(helpText)
		return
	}
	if *showAgent {
		fmt.Print(agentGuide)
		return
	}

	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	catalog, err := store.OpenCatalog(cfg.DataDir)
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}
	defer catalog.Close()

	if cfg.BackfillOnStart && cfg.GitHubToken != "" {
		runner := &backfill.Runner{
			Catalog: catalog,
			Lister: &backfill.GitHubAppClient{
				BaseURL: cfg.GitHubAPI,
				Token: func(ctx context.Context, repoID int64) (string, error) {
					return cfg.GitHubToken, nil
				},
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := runner.RunForDiscovered(ctx); err != nil {
			log.Printf("backfill (non-fatal): %v", err)
		}
		cancel()
	}

	mux := http.NewServeMux()
	mux.Handle("/webhook/github", &webhook.Handler{
		Secret:  cfg.WebhookSecret,
		Catalog: catalog,
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("issuepipe listening on %s (data=%s)", cfg.ListenAddr, cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

const helpText = `issuepipe — Fly-hosted sqlpipe Master for GitHub issue webhooks

USAGE:
    issuepipe                 Start the HTTP server
    issuepipe --version
    issuepipe --help
    issuepipe --help-agent

ENVIRONMENT:
    LISTEN_ADDR               Bind address (default :8080)
    DATA_DIR                  Per-repo SQLite Master directory (default /data)
    GITHUB_WEBHOOK_SECRET     Required. HMAC secret for X-Hub-Signature-256
    GITHUB_TOKEN              Optional. Token for cold-start backfill poll
    GITHUB_API                API base (default https://api.github.com)
    BACKFILL_ON_START         true/false (default true when token set)

Endpoints:
    POST /webhook/github      GitHub App issues webhooks
    GET  /health              Liveness
`

const agentGuide = `# issuepipe agent guide

issuepipe is the Stage-1 service for bullseye's GitHub-issues integration
(🎯T31): a Fly-hosted sqlpipe Master that ingests GitHub App issue webhooks
into per-repo SQLite tables.

## Layout

- cmd/issuepipe — process entry
- internal/webhook — HMAC verify + issues event upsert
- internal/store — per-repo SQLite + sqlpipe Master catalog
- internal/backfill — cold-start GitHub list-issues poll (only reconcile path)
- internal/config — env config

## Invariants

- Keys are stable GitHub ids (repo id, issue node_id) — never owner/name.
- Upserts are idempotent under webhook redelivery.
- One DB file per repo so T32 can authorize at ownership granularity.
- No steady-state poll: webhooks deliver; backfill only on cold start / lost volume.

## Build

    make test
    make build   # needs CGO + C++ for sqlpipe

## Related

- bullseye 🎯T31–T35 (integration graph lives in bullseye until fully relocated)
- sqlpipe Go binding: github.com/marcelocantos/sqlpipe/go/sqlpipe
`
