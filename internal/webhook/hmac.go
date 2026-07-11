// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
)

// VerifySignature checks GitHub's X-Hub-Signature-256 header against body
// using the webhook secret. Header form: "sha256=<hex>".
func VerifySignature(secret string, body []byte, header string) error {
	if secret == "" {
		return fmt.Errorf("webhook secret is empty")
	}
	header = strings.TrimSpace(header)
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return fmt.Errorf("signature header missing sha256= prefix")
	}
	gotHex := strings.TrimPrefix(header, prefix)
	got, err := hex.DecodeString(gotHex)
	if err != nil {
		return fmt.Errorf("signature header not hex: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	want := mac.Sum(nil)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// SignBody produces an X-Hub-Signature-256 value for tests.
func SignBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
