// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveColorSpec(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want string
	}{
		{"empty", "", ""},
		{"named", "green", sgrGreen},
		{"combined", "bold green", sgrBold + sgrGreen},
		{"case insensitive", "BOLD Red", sgrBold + sgrRed},
		{"256 color", "256:130", "\033[38;5;130m"},
		{"named plus 256", "bold 256:46", sgrBold + "\033[38;5;46m"},
		{"raw escape passthrough", "\033[38;5;200m", "\033[38;5;200m"},
		{"unknown token skipped", "green notacolor", sgrGreen},
		{"out of range 256 skipped", "256:999", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveColorSpec(tt.spec); got != tt.want {
				t.Errorf("resolveColorSpec(%q) = %q, want %q", tt.spec, got, tt.want)
			}
		})
	}
}

func TestLoadThemeFileJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mytheme.json")
	contents := `{
		"name": "solarized",
		"colors": {
			"success": "256:46",
			"error": "bold red",
			"info": "cyan"
		}
	}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	theme, err := LoadThemeFile(path)
	if err != nil {
		t.Fatalf("LoadThemeFile() error = %v", err)
	}
	if theme != Theme("solarized") {
		t.Fatalf("LoadThemeFile() theme = %q, want %q", theme, "solarized")
	}

	original := GetTheme()
	defer SetTheme(original)
	SetTheme(theme)

	if got := themeColors("success"); got != "\033[38;5;46m" {
		t.Errorf("success = %q, want %q", got, "\033[38;5;46m")
	}
	if got := themeColors("error"); got != sgrBold+sgrRed {
		t.Errorf("error = %q, want %q", got, sgrBold+sgrRed)
	}
	if got := themeColors("info"); got != sgrCyan {
		t.Errorf("info = %q, want %q", got, sgrCyan)
	}
	// Undefined semantic falls back to the default palette.
	if got := themeColors("warning"); got != sgrYellow {
		t.Errorf("warning fallback = %q, want %q", got, sgrYellow)
	}
}

func TestLoadThemeFileYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mytheme.yaml")
	contents := "name: ocean\ncolors:\n  success: \"bold cyan\"\n  error: magenta\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	theme, err := LoadThemeFile(path)
	if err != nil {
		t.Fatalf("LoadThemeFile() error = %v", err)
	}

	original := GetTheme()
	defer SetTheme(original)
	SetTheme(theme)

	if got := themeColors("success"); got != sgrBold+sgrCyan {
		t.Errorf("success = %q, want %q", got, sgrBold+sgrCyan)
	}
	if got := themeColors("error"); got != sgrMagenta {
		t.Errorf("error = %q, want %q", got, sgrMagenta)
	}
}

func TestLoadThemeFileErrors(t *testing.T) {
	dir := t.TempDir()

	t.Run("missing file", func(t *testing.T) {
		if _, err := LoadThemeFile(filepath.Join(dir, "nope.json")); err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		path := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadThemeFile(path); err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		path := filepath.Join(dir, "noname.json")
		if err := os.WriteFile(path, []byte(`{"colors":{"success":"green"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadThemeFile(path); err == nil {
			t.Error("expected error for missing name")
		}
	})
}

func TestLoadAndSetTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.json")
	if err := os.WriteFile(path, []byte(`{"name":"loadset","colors":{"info":"green"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	original := GetTheme()
	defer SetTheme(original)

	if err := LoadAndSetTheme(path); err != nil {
		t.Fatalf("LoadAndSetTheme() error = %v", err)
	}
	if GetTheme() != Theme("loadset") {
		t.Errorf("active theme = %q, want %q", GetTheme(), "loadset")
	}
	if got := themeColors("info"); got != sgrGreen {
		t.Errorf("info = %q, want %q", got, sgrGreen)
	}
}

func TestDetectThemeFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.json")
	if err := os.WriteFile(path, []byte(`{"name":"envtheme","colors":{"success":"green"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	os.Setenv("ERST_THEME_FILE", path)
	defer os.Unsetenv("ERST_THEME_FILE")

	if got := DetectTheme(); got != Theme("envtheme") {
		t.Errorf("DetectTheme() = %q, want %q", got, "envtheme")
	}

	// A bad path is ignored and detection continues.
	os.Setenv("ERST_THEME_FILE", filepath.Join(dir, "missing.json"))
	os.Unsetenv("ERST_THEME")
	os.Setenv("COLORTERM", "truecolor")
	defer os.Unsetenv("COLORTERM")
	if got := DetectTheme(); got != ThemeDefault {
		t.Errorf("DetectTheme() with bad file = %q, want %q", got, ThemeDefault)
	}
}
