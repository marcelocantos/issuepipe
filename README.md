# issuepipe

Fly-hosted **sqlpipe Master** that ingests GitHub App **issue webhooks** into
per-repo SQLite tables.

This is Stage 1 of the bullseye GitHub-issues integration (🎯T31):

```
GitHub App webhooks → issuepipe (Fly Master) → sqlpipe Replicas (localhost)
```

Per-repo table/DB partitioning is intentional: later auth (🎯T32) scopes each
Replica to the connecting user's GitHub-readable repos at ownership
granularity inside the convergence boundary.

## Status

v0.1.0 — local service + tests for HMAC verification, idempotent upserts,
per-repo Master files, and cold-start backfill. Fly deploy and App
installation token exchange land next.

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
