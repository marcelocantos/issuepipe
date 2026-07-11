// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package webhook

import "testing"

func TestVerifySignatureOK(t *testing.T) {
	body := []byte(`{"zen":"Design for failure."}`)
	secret := "test-secret"
	sig := SignBody(secret, body)
	if err := VerifySignature(secret, body, sig); err != nil {
		t.Fatal(err)
	}
}

func TestVerifySignatureRejectsMissingPrefix(t *testing.T) {
	err := VerifySignature("s", []byte("x"), "deadbeef")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifySignatureRejectsMismatch(t *testing.T) {
	body := []byte(`{"a":1}`)
	sig := SignBody("correct", body)
	err := VerifySignature("wrong", body, sig)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestVerifySignatureRejectsEmptySecret(t *testing.T) {
	err := VerifySignature("", []byte("x"), "sha256=00")
	if err == nil {
		t.Fatal("expected empty secret error")
	}
}
