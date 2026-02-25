// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import "os"

// Theme defines a color palette for terminal output
type Theme string

const (
	ThemeDefault      Theme = "default"
	ThemeDeuteranopia Theme = "deuteranopia"
	ThemeProtanopia   Theme = "protanopia"
	ThemeTritanopia   Theme = "tritanopia"
	ThemeHighContrast Theme = "high-contrast"
)

var currentTheme = ThemeDefault

// SetTheme configures the active color theme
func SetTheme(theme Theme) {
	currentTheme = theme
}

// GetTheme returns the currently active theme
func GetTheme() Theme {
	return currentTheme
}

// DetectTheme attempts to detect an appropriate theme from environment
func DetectTheme() Theme {
	if theme := os.Getenv("ERST_THEME"); theme != "" {
		return Theme(theme)
	}
	if os.Getenv("COLORTERM") == "truecolor" {
		return ThemeDefault
	}
	return ThemeHighContrast
}

// themeColors maps semantic color names to ANSI codes per theme
func themeColors(semantic string) string {
	switch currentTheme {
	case ThemeDeuteranopia, ThemeProtanopia:
		// Red-green color blindness: use blue/yellow/cyan
		switch semantic {
		case "success":
			return sgrCyan
		case "error":
			return sgrMagenta
		case "warning":
			return sgrYellow
		case "info":
			return sgrBlue
		case "dim":
			return sgrDim
		case "bold":
			return sgrBold
		default:
			return ""
		}
	case ThemeTritanopia:
		// Blue-yellow color blindness: use red/green/magenta
		switch semantic {
		case "success":
			return sgrGreen
		case "error":
			return sgrRed
		case "warning":
			return sgrMagenta
		case "info":
			return sgrCyan
		case "dim":
			return sgrDim
		case "bold":
			return sgrBold
		default:
			return ""
		}
	case ThemeHighContrast:
		// High contrast: bold colors only
		switch semantic {
		case "success":
			return sgrBold + sgrGreen
		case "error":
			return sgrBold + sgrRed
		case "warning":
			return sgrBold + sgrYellow
		case "info":
			return sgrBold + sgrCyan
		case "dim":
			return ""
		case "bold":
			return sgrBold
		default:
			return ""
		}
	default:
		// Default theme
		switch semantic {
		case "success":
			return sgrGreen
		case "error":
			return sgrRed
		case "warning":
			return sgrYellow
		case "info":
			return sgrBlue
		case "dim":
			return sgrDim
		case "bold":
			return sgrBold
		default:
			return ""
		}
	}
}
