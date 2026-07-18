# issuepipe

Fly-hosted **sqlpipe Master** that ingests GitHub **issue webhooks** into
per-repo SQLite tables.

This is Stage 1 of the bullseye GitHub-issues integration (🎯T31):

```
Repo webhooks → issuepipe (Fly Master) → sqlpipe Replicas (localhost)
```

**Transport decision (2026-07-18):** Stage 1 uses **repo webhooks** (agent can
register them via `gh`). A GitHub App is deferred until bullseye 🎯T32
(user auth / installation tokens). Same `POST /webhook/github` + HMAC
either way.

Per-repo table/DB partitioning is intentional: later auth (🎯T32) scopes each
Replica to the connecting user's GitHub-readable repos at ownership
granularity inside the convergence boundary.

## Status

Live: https://issuepipe.fly.dev — health + HMAC-verified webhooks.
v0.1.0 tests cover signature reject, idempotent upserts, per-repo Masters,
and cold-start backfill.

## Quick start

```bash
export GITHUB_WEBHOOK_SECRET=dev-secret
export DATA_DIR=./data
export BACKFILL_ON_START=false
make build && ./bin/issuepipe
```

Webhook endpoint: `POST /webhook/github` with `X-Hub-Signature-256` and
`X-GitHub-Event: issues`.

## Environment

| Variable | Required | Default | Meaning |
|----------|----------|---------|---------|
| `GITHUB_WEBHOOK_SECRET` | yes | — | HMAC secret |
| `DATA_DIR` | no | `/data` | Per-repo `repo_<id>.db` directory |
| `LISTEN_ADDR` | no | `:8080` | HTTP bind |
| `GITHUB_TOKEN` | no | — | Token for cold-start backfill |
| `GITHUB_API` | no | `https://api.github.com` | API base |
| `BACKFILL_ON_START` | no | `true` | Run list-issues poll on boot |

## License

Apache-2.0
