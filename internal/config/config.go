// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config is process configuration loaded from the environment.
type Config struct {
	// ListenAddr is the HTTP bind address (default :8080).
	ListenAddr string
	// DataDir holds per-repo SQLite Master files.
	DataDir string
	// WebhookSecret is the GitHub App webhook HMAC secret.
	WebhookSecret string
	// BackfillOnStart runs GitHub issue list poll after open (default true
	// when a token path is configured; false in tests).
	BackfillOnStart bool
	// GitHubToken is an optional PAT/installation token for backfill.
	// Production should use App installation tokens (T32); this is v0.
	GitHubToken string
	// GitHubAPI is the API base URL (default https://api.github.com).
	GitHubAPI string
}

// FromEnv loads Config from environment variables.
func FromEnv() (Config, error) {
	c := Config{
		ListenAddr:      envOr("LISTEN_ADDR", ":8080"),
		DataDir:         envOr("DATA_DIR", "/data"),
		WebhookSecret:   os.Getenv("GITHUB_WEBHOOK_SECRET"),
		GitHubToken:     os.Getenv("GITHUB_TOKEN"),
		GitHubAPI:       envOr("GITHUB_API", "https://api.github.com"),
		BackfillOnStart: true,
	}
	if v := os.Getenv("BACKFILL_ON_START"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("BACKFILL_ON_START: %w", err)
		}
		c.BackfillOnStart = b
	}
	if c.WebhookSecret == "" {
		return Config{}, fmt.Errorf("GITHUB_WEBHOOK_SECRET is required")
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
