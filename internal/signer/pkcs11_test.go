// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestPkcs11Config_RedactsPIN(t *testing.T) {
	secretPIN := "super-secret-pin-987654"
	cfg := Pkcs11Config{
		ModulePath: "/usr/lib/softhsm/libsofthsm2.so",
		PIN:        secretPIN,
		TokenLabel: "MyToken",
		SlotIndex:  3,
		KeyLabel:   "MySigningKey",
		KeyIDHex:   "abcd0123",
	}

	formats := []struct {
		name   string
		format string
		arg    any
	}{
		{"percent_v_value", "%v", cfg},
		{"percent_plus_v_value", "%+v", cfg},
		{"percent_hash_v_value", "%#v", cfg},
		{"percent_s_value", "%s", cfg},
		{"string_method", "", cfg.String()},
		{"percent_v_pointer", "%v", &cfg},
		{"percent_plus_v_pointer", "%+v", &cfg},
		{"percent_hash_v_pointer", "%#v", &cfg},
		{"percent_s_pointer", "%s", &cfg},
	}

	for _, tt := range formats {
		t.Run(tt.name, func(t *testing.T) {
			var out string
			if tt.format == "" {
				out = tt.arg.(string)
			} else {
				out = fmt.Sprintf(tt.format, tt.arg)
			}

			if strings.Contains(out, secretPIN) {
				t.Fatalf("format %q leaked secret PIN: %s", tt.name, out)
			}
			if !strings.Contains(out, "[REDACTED]") {
				t.Fatalf("format %q does not contain [REDACTED]: %s", tt.name, out)
			}
			if !strings.Contains(out, "/usr/lib/softhsm/libsofthsm2.so") {
				t.Fatalf("format %q missing ModulePath: %s", tt.name, out)
			}
		})
	}
}

func TestPkcs11Config_SlogLogValueRedaction(t *testing.T) {
	secretPIN := "vault-hsm-pin-4321"
	cfg := Pkcs11Config{
		ModulePath: "/opt/hsm/libpkcs11.so",
		PIN:        secretPIN,
		TokenLabel: "SecureSlot",
		SlotIndex:  0,
		KeyLabel:   "stellar-key",
		KeyIDHex:   "112233",
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	logger.Info("pkcs11 configuration initialized", "config", cfg)

	logOutput := buf.String()
	if strings.Contains(logOutput, secretPIN) {
		t.Fatalf("slog JSON output leaked secret PIN: %s", logOutput)
	}
	if !strings.Contains(logOutput, "[REDACTED]") {
		t.Fatalf("slog JSON output does not contain [REDACTED]: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/opt/hsm/libpkcs11.so") {
		t.Fatalf("slog JSON output missing module path: %s", logOutput)
	}
}

func TestPkcs11ConfigFromEnv_ValidationAndRedaction(t *testing.T) {
	// 1. Missing module path
	os.Unsetenv("ERST_PKCS11_MODULE")
	os.Unsetenv("ERST_PKCS11_PIN")
	_, err := Pkcs11ConfigFromEnv()
	if err == nil {
		t.Fatalf("expected error when ERST_PKCS11_MODULE is unset")
	}

	// 2. Missing PIN
	os.Setenv("ERST_PKCS11_MODULE", "/usr/lib/libsofthsm2.so")
	_, err = Pkcs11ConfigFromEnv()
	if err == nil {
		t.Fatalf("expected error when ERST_PKCS11_PIN is unset")
	}

	// 3. Valid env variables
	secretPIN := "topsecretpin-env"
	os.Setenv("ERST_PKCS11_PIN", secretPIN)
	os.Setenv("ERST_PKCS11_TOKEN_LABEL", "EnvToken")
	os.Setenv("ERST_PKCS11_KEY_LABEL", "EnvKey")
	os.Setenv("ERST_PKCS11_KEY_ID", "aabbcc")
	defer func() {
		os.Unsetenv("ERST_PKCS11_MODULE")
		os.Unsetenv("ERST_PKCS11_PIN")
		os.Unsetenv("ERST_PKCS11_TOKEN_LABEL")
		os.Unsetenv("ERST_PKCS11_KEY_LABEL")
		os.Unsetenv("ERST_PKCS11_KEY_ID")
	}()

	cfg, err := Pkcs11ConfigFromEnv()
	if err != nil {
		t.Fatalf("Pkcs11ConfigFromEnv failed: %v", err)
	}

	formatted := fmt.Sprintf("%+v", cfg)
	if strings.Contains(formatted, secretPIN) {
		t.Fatalf("formatted env config leaked secret PIN: %s", formatted)
	}
	if !strings.Contains(formatted, "[REDACTED]") {
		t.Fatalf("formatted env config does not contain [REDACTED]: %s", formatted)
	}
}
