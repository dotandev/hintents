// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

func TestInMemorySignerFromSeed(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("key generation failed: %v", err)
	}
	seedHex := hex.EncodeToString(priv.Seed())

	s, err := NewInMemorySigner(seedHex)
	if err != nil {
		t.Fatalf("NewInMemorySigner failed: %v", err)
	}

	if s.Algorithm() != "ed25519" {
		t.Fatalf("unexpected algorithm: %s", s.Algorithm())
	}

	gotPub, err := s.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey failed: %v", err)
	}
	if hex.EncodeToString(gotPub) != hex.EncodeToString(pub) {
		t.Fatalf("public key mismatch")
	}
}

func TestInMemorySignerFromFullKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	fullHex := hex.EncodeToString(priv)

	s, err := NewInMemorySigner(fullHex)
	if err != nil {
		t.Fatalf("NewInMemorySigner (full key) failed: %v", err)
	}

	data := []byte("test payload")
	sig, err := s.Sign(data)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	pub, _ := s.PublicKey()
	if !ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
		t.Fatal("signature verification failed")
	}
}

func TestInMemorySignerRoundTrip(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	s := NewInMemorySignerFromKey(priv)

	data := []byte("audit trail hash")
	sig, err := s.Sign(data)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	pub, _ := s.PublicKey()
	if !ed25519.Verify(ed25519.PublicKey(pub), data, sig) {
		t.Fatal("round-trip verification failed")
	}
}

func TestInMemorySignerInvalidHex(t *testing.T) {
	_, err := NewInMemorySigner("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestInMemorySignerWrongKeyLength(t *testing.T) {
	_, err := NewInMemorySigner("aabb")
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestSignerErrorFormat(t *testing.T) {
	e := &SignerError{Op: "test", Msg: "something failed"}
	if e.Error() != "test: something failed" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}
}

func TestSignerErrorUnwrap(t *testing.T) {
	inner := &SignerError{Op: "inner", Msg: "root cause"}
	outer := &SignerError{Op: "outer", Msg: "wrapping", Err: inner}
	if outer.Unwrap() != inner {
		t.Fatal("Unwrap did not return inner error")
	}
}

func TestSignerInterfaceSatisfied(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	var s Signer = NewInMemorySignerFromKey(priv)

	if s.Algorithm() != "ed25519" {
		t.Fatalf("interface method returned unexpected algorithm: %s", s.Algorithm())
	}
}

func TestPkcs11ConfigFromEnv_MissingModule(t *testing.T) {
	t.Setenv("ERST_PKCS11_MODULE", "")
	t.Setenv("ERST_PKCS11_PIN", "1234")

	_, err := Pkcs11ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error when ERST_PKCS11_MODULE is empty")
	}
}

func TestPkcs11ConfigFromEnv_MissingPIN(t *testing.T) {
	t.Setenv("ERST_PKCS11_MODULE", "/usr/lib/softhsm/libsofthsm2.so")
	t.Setenv("ERST_PKCS11_PIN", "")

	_, err := Pkcs11ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error when ERST_PKCS11_PIN is empty")
	}
}

func TestPkcs11ConfigFromEnv_ValidConfig(t *testing.T) {
	t.Setenv("ERST_PKCS11_MODULE", "/usr/lib/softhsm/libsofthsm2.so")
	t.Setenv("ERST_PKCS11_PIN", "1234")
	t.Setenv("ERST_PKCS11_TOKEN_LABEL", "MyToken")
	t.Setenv("ERST_PKCS11_KEY_LABEL", "signing-key")
	t.Setenv("ERST_PKCS11_KEY_ID", "aabb")

	cfg, err := Pkcs11ConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ModulePath != "/usr/lib/softhsm/libsofthsm2.so" {
		t.Fatalf("unexpected module path: %s", cfg.ModulePath)
	}
	if cfg.PIN != "1234" {
		t.Fatalf("unexpected PIN")
	}
	if cfg.TokenLabel != "MyToken" {
		t.Fatalf("unexpected token label: %s", cfg.TokenLabel)
	}
	if cfg.KeyLabel != "signing-key" {
		t.Fatalf("unexpected key label: %s", cfg.KeyLabel)
	}
	if cfg.KeyIDHex != "aabb" {
		t.Fatalf("unexpected key ID hex: %s", cfg.KeyIDHex)
	}
}
