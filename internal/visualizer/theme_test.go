// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"os"
	"testing"
)

func TestSetTheme(t *testing.T) {
	tests := []struct {
		name  string
		theme Theme
		want  Theme
	}{
		{"default", ThemeDefault, ThemeDefault},
		{"deuteranopia", ThemeDeuteranopia, ThemeDeuteranopia},
		{"protanopia", ThemeProtanopia, ThemeProtanopia},
		{"tritanopia", ThemeTritanopia, ThemeTritanopia},
		{"high-contrast", ThemeHighContrast, ThemeHighContrast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTheme(tt.theme)
			if got := GetTheme(); got != tt.want {
				t.Errorf("GetTheme() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectTheme(t *testing.T) {
	tests := []struct {
		name      string
		envTheme  string
		colorTerm string
		want      Theme
	}{
		{"explicit theme", "deuteranopia", "", ThemeDeuteranopia},
		{"truecolor", "", "truecolor", ThemeDefault},
		{"fallback", "", "", ThemeHighContrast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Unsetenv("ERST_THEME")
			_ = os.Unsetenv("COLORTERM")

			if tt.envTheme != "" {
				_ = os.Setenv("ERST_THEME", tt.envTheme)
			}
			if tt.colorTerm != "" {
				_ = os.Setenv("COLORTERM", tt.colorTerm)
			}

			if got := DetectTheme(); got != tt.want {
				t.Errorf("DetectTheme() = %v, want %v", got, tt.want)
			}

			_ = os.Unsetenv("ERST_THEME")
			_ = os.Unsetenv("COLORTERM")
		})
	}
}

func TestThemeColors(t *testing.T) {
	tests := []struct {
		name     string
		theme    Theme
		semantic string
		wantCode string
	}{
		{"default success", ThemeDefault, "success", sgrGreen},
		{"default error", ThemeDefault, "error", sgrRed},
		{"default warning", ThemeDefault, "warning", sgrYellow},
		{"deuteranopia success", ThemeDeuteranopia, "success", sgrCyan},
		{"deuteranopia error", ThemeDeuteranopia, "error", sgrMagenta},
		{"protanopia success", ThemeProtanopia, "success", sgrCyan},
		{"tritanopia success", ThemeTritanopia, "success", sgrGreen},
		{"tritanopia warning", ThemeTritanopia, "warning", sgrMagenta},
		{"high-contrast success", ThemeHighContrast, "success", sgrBold + sgrGreen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTheme(tt.theme)
			if got := themeColors(tt.semantic); got != tt.wantCode {
				t.Errorf("themeColors(%q) = %q, want %q", tt.semantic, got, tt.wantCode)
			}
		})
	}
}

func TestThemeAwareIndicators(t *testing.T) {
	originalTheme := GetTheme()
	defer SetTheme(originalTheme)

	_ = os.Setenv("FORCE_COLOR", "1")
	defer func() { _ = os.Unsetenv("FORCE_COLOR") }()

	tests := []struct {
		name  string
		theme Theme
		fn    func() string
	}{
		{"success default", ThemeDefault, Success},
		{"success deuteranopia", ThemeDeuteranopia, Success},
		{"error default", ThemeDefault, Error},
		{"warning high-contrast", ThemeHighContrast, Warning},
		{"info default", ThemeDefault, Info},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTheme(tt.theme)
			result := tt.fn()
			if result == "" {
				t.Error("indicator returned empty string")
			}
			if !ColorEnabled() {
				t.Skip("colors disabled")
			}
		})
	}
}
