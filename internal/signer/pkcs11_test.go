// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package signer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const testSecretPIN = "s3cr3t-P1N-98765"

func testPkcs11Config() Pkcs11Config {
	return Pkcs11Config{
		ModulePath: "/usr/lib/softhsm/libsofthsm2.so",
		PIN:        testSecretPIN,
		TokenLabel: "my-token",
		SlotIndex:  7,
		KeyLabel:   "signing-key",
		KeyIDHex:   "deadbeef",
	}
}

func assertRedacted(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, testSecretPIN) {
		t.Fatalf("PIN leaked in output: %q", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker in output: %q", out)
	}
	for _, want := range []string{
		"/usr/lib/softhsm/libsofthsm2.so", "my-token", "7", "signing-key", "deadbeef",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q to be preserved in output: %q", want, out)
		}
	}
}

func TestPkcs11Config_FormatVerbs(t *testing.T) {
	cfg := testPkcs11Config()
	ptr := &cfg

	targets := map[string]any{
		"value":   cfg,
		"pointer": ptr,
	}
	verbs := []string{"%v", "%+v", "%s", "%#v"}

	for name, target := range targets {
		for _, verb := range verbs {
			t.Run(name+"/"+verb, func(t *testing.T) {
				assertRedacted(t, fmt.Sprintf(verb, target))
			})
		}
	}
}

func TestPkcs11Config_Sprint(t *testing.T) {
	cfg := testPkcs11Config()
	assertRedacted(t, fmt.Sprint(cfg))
	assertRedacted(t, fmt.Sprint(&cfg))
	assertRedacted(t, fmt.Sprintln(cfg))
}

func TestPkcs11Config_StringMethod(t *testing.T) {
	cfg := testPkcs11Config()
	assertRedacted(t, cfg.String())
	assertRedacted(t, (&cfg).String())
}

func TestPkcs11Config_NestedInStruct(t *testing.T) {
	cfg := testPkcs11Config()
	wrapper := struct {
		Name string
		Cfg  Pkcs11Config
		Ptr  *Pkcs11Config
	}{"w", cfg, &cfg}

	for _, verb := range []string{"%v", "%+v"} {
		out := fmt.Sprintf(verb, wrapper)
		if strings.Contains(out, "s3cr3t") {
			t.Fatalf("PIN leaked for nested %s: %q", verb, out)
		}
	}
}

func TestPkcs11Config_NilPointerDoesNotPanic(t *testing.T) {
	var cfg *Pkcs11Config
	out := fmt.Sprintf("%v", cfg)
	if strings.Contains(out, testSecretPIN) {
		t.Fatalf("unexpected output: %q", out)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	logger.Info("nil config", "cfg", cfg)
	if strings.Contains(buf.String(), testSecretPIN) {
		t.Fatalf("PIN leaked: %q", buf.String())
	}
}

func TestPkcs11Config_EmptyPINStillRedacted(t *testing.T) {
	cfg := Pkcs11Config{ModulePath: "/m.so"}
	out := cfg.String()
	if !strings.Contains(out, "PIN:[REDACTED]") {
		t.Fatalf("expected redacted PIN marker: %q", out)
	}
}

func TestPkcs11Config_SlogText(t *testing.T) {
	cfg := testPkcs11Config()

	for name, val := range map[string]any{"value": cfg, "pointer": &cfg} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			logger.Info("config loaded", "cfg", val)

			out := buf.String()
			assertRedacted(t, out)
			if !strings.Contains(out, "cfg.pin=[REDACTED]") {
				t.Fatalf("expected cfg.pin=[REDACTED] in %q", out)
			}
		})
	}
}

func TestPkcs11Config_SlogJSON(t *testing.T) {
	cfg := testPkcs11Config()

	for name, val := range map[string]any{"value": cfg, "pointer": &cfg} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			logger.Info("config loaded", slog.Any("cfg", val))

			out := buf.String()
			if strings.Contains(out, testSecretPIN) {
				t.Fatalf("PIN leaked: %q", out)
			}

			var rec struct {
				Cfg map[string]any `json:"cfg"`
			}
			if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
				t.Fatalf("invalid JSON log: %v", err)
			}
			if rec.Cfg["pin"] != "[REDACTED]" {
				t.Fatalf("pin = %v, want [REDACTED]", rec.Cfg["pin"])
			}
			if rec.Cfg["module_path"] != cfg.ModulePath ||
				rec.Cfg["token_label"] != cfg.TokenLabel ||
				rec.Cfg["key_label"] != cfg.KeyLabel ||
				rec.Cfg["key_id_hex"] != cfg.KeyIDHex ||
				rec.Cfg["slot_index"] != float64(cfg.SlotIndex) {
				t.Fatalf("non-sensitive fields not preserved: %v", rec.Cfg)
			}
		})
	}
}

func TestPkcs11Config_SlogWithAttrsAndGroup(t *testing.T) {
	cfg := testPkcs11Config()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil)).With("cfg", &cfg).WithGroup("g")
	logger.Warn("msg", "inner", cfg)

	if strings.Contains(buf.String(), testSecretPIN) {
		t.Fatalf("PIN leaked: %q", buf.String())
	}
}

func TestPkcs11Config_LogValueDirect(t *testing.T) {
	cfg := testPkcs11Config()
	v := cfg.LogValue()
	if v.Kind() != slog.KindGroup {
		t.Fatalf("kind = %v, want group", v.Kind())
	}
	for _, a := range v.Group() {
		if a.Key == "pin" && a.Value.String() != "[REDACTED]" {
			t.Fatalf("pin attr = %q", a.Value.String())
		}
		if strings.Contains(a.Value.String(), testSecretPIN) {
			t.Fatalf("PIN leaked in attr %s", a.Key)
		}
	}
}

func TestPkcs11Config_OriginalPINUnchanged(t *testing.T) {
	cfg := testPkcs11Config()
	_ = fmt.Sprintf("%v %+v %s", cfg, cfg, &cfg)
	if cfg.PIN != testSecretPIN {
		t.Fatalf("PIN was mutated: %q", cfg.PIN)
	}
}
