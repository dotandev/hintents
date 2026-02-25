// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"os"
	"testing"
)

func TestColorBlindThemes(t *testing.T) {
	os.Setenv("FORCE_COLOR", "1")
	defer os.Unsetenv("FORCE_COLOR")

	tests := []struct {
		name  string
		theme Theme
	}{
		{"deuteranopia", ThemeDeuteranopia},
		{"protanopia", ThemeProtanopia},
		{"tritanopia", ThemeTritanopia},
		{"high-contrast", ThemeHighContrast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetTheme(tt.theme)

			success := Success()
			if success == "" {
				t.Error("Success() returned empty string")
			}

			warning := Warning()
			if warning == "" {
				t.Error("Warning() returned empty string")
			}

			errorMsg := Error()
			if errorMsg == "" {
				t.Error("Error() returned empty string")
			}

			info := Info()
			if info == "" {
				t.Error("Info() returned empty string")
			}
		})
	}
}

func TestThemeConsistency(t *testing.T) {
	os.Setenv("FORCE_COLOR", "1")
	defer os.Unsetenv("FORCE_COLOR")

	themes := []Theme{
		ThemeDefault,
		ThemeDeuteranopia,
		ThemeProtanopia,
		ThemeTritanopia,
		ThemeHighContrast,
	}

	for _, theme := range themes {
		t.Run(string(theme), func(t *testing.T) {
			SetTheme(theme)

			successColor := themeColors("success")
			errorColor := themeColors("error")
			warningColor := themeColors("warning")

			if successColor == errorColor {
				t.Error("success and error colors should be different")
			}
			if successColor == warningColor {
				t.Error("success and warning colors should be different")
			}
		})
	}
}
