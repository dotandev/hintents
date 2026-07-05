// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestReadAuditPayload_File(t *testing.T) {
	content := `{"foo":"bar"}`
	file, err := os.CreateTemp("", "payload-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
	file.Close()

	bytes, err := readAuditPayload("", file.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(bytes) != content {
		t.Fatalf("expected payload %q, got %q", content, string(bytes))
	}
}

func TestResolveAuditSigner_SoftwarePEM_Env(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	pemBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pemBytes})

	t.Setenv("ERST_AUDIT_PRIVATE_KEY_PEM", string(pemData))
	auditSignSoftwareKey = ""
	auditSignHSMProvider = ""

	signerImpl, err := resolveAuditSigner()
	if err != nil {
		t.Fatalf("resolveAuditSigner failed: %v", err)
	}

	msg := []byte("test")
	sig, err := signerImpl.Sign(msg)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	pub, err := signerImpl.PublicKey()
	if err != nil {
		t.Fatalf("public key failed: %v", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		t.Fatal("signature verification failed")
	}
}

func TestRunAuditSign_OutputsJSON(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	pemBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemData := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pemBytes})

	auditSignPayload = `{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}`
	auditSignPayloadFile = ""
	auditSignSoftwareKey = string(pemData)
	auditSignHSMProvider = ""

	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	if err := runAuditSign(cmd, nil); err != nil {
		t.Fatalf("runAuditSign failed: %v", err)
	}

	var log SignedAuditLog
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}
	if log.TraceHash == "" || log.Signature == "" || log.PublicKey == "" {
		t.Fatalf("missing signed fields: %+v", log)
	}
	if !strings.Contains(string(log.Payload), "\"input\"") {
		t.Fatalf("payload not preserved in output")
	}
}
