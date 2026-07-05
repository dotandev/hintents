// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package visualizer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Theme defines a color palette for terminal output
type Theme string

const (
	ThemeDefault      Theme = "default"
	ThemeDeuteranopia Theme = "deuteranopia"
	ThemeProtanopia   Theme = "protanopia"
	ThemeTritanopia   Theme = "tritanopia"
	ThemeHighContrast Theme = "high-contrast"
	ThemeLight        Theme = "light"
	ThemeDark         Theme = "dark"
)

// ThemeDefinition describes a custom theme loaded from a configuration file.
//
// Each entry in Colors maps a semantic slot (success, error, warning, info,
// dim, bold) to a color specification. A specification is one or more
// space-separated tokens, each of which is either:
//   - a named attribute: red, green, yellow, blue, magenta, cyan, bold, dim
//   - a 256-color code in the form "256:N" (0-255), e.g. "256:130"
//   - a raw ANSI SGR escape sequence (anything containing an ESC byte)
//
// Tokens are concatenated, so "bold green" yields bold + green output.
type ThemeDefinition struct {
	Name   string            `json:"name" yaml:"name"`
	Colors map[string]string `json:"colors" yaml:"colors"`
}

// customThemes holds themes registered at runtime, keyed by name. Entries here
// take precedence over the built-in palettes, allowing both new user themes and
// overrides of the built-ins.
var customThemes = map[Theme]map[string]string{}

var currentTheme = ThemeDefault

// SetTheme configures the active color theme
func SetTheme(theme Theme) {
	currentTheme = theme
}

// GetTheme returns the currently active theme
func GetTheme() Theme {
	return currentTheme
}

// DetectTheme attempts to detect an appropriate theme from environment.
//
// If ERST_THEME_FILE points at a readable JSON/YAML theme file, that theme is
// loaded, registered, and returned. Otherwise the legacy ERST_THEME /
// COLORTERM detection is used.
func DetectTheme() Theme {
	if path := os.Getenv("ERST_THEME_FILE"); path != "" {
		if theme, err := LoadThemeFile(path); err == nil {
			return theme
		}
	}
	if theme := os.Getenv("ERST_THEME"); theme != "" {
		switch theme {
		case "light":
			return ThemeLight
		case "dark":
			return ThemeDark
		default:
			return Theme(theme)
		}
	}
	if os.Getenv("COLORTERM") == "truecolor" {
		return ThemeDefault
	}
	return ThemeHighContrast
}

// RegisterTheme registers a custom theme under name, resolving each color
// specification into its ANSI escape codes. A registered theme overrides any
// built-in theme sharing the same name.
func RegisterTheme(name string, colors map[string]string) {
	resolved := make(map[string]string, len(colors))
	for semantic, spec := range colors {
		resolved[strings.ToLower(strings.TrimSpace(semantic))] = resolveColorSpec(spec)
	}
	customThemes[Theme(name)] = resolved
}

// LoadThemeFile reads a theme definition from a JSON or YAML configuration file,
// registers it, and returns the registered theme name. The format is selected
// by file extension (.yaml/.yml is YAML, anything else is JSON).
func LoadThemeFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading theme file: %w", err)
	}

	var def ThemeDefinition
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &def); err != nil {
			return "", fmt.Errorf("parsing YAML theme %q: %w", path, err)
		}
	default:
		if err := json.Unmarshal(data, &def); err != nil {
			return "", fmt.Errorf("parsing JSON theme %q: %w", path, err)
		}
	}

	if strings.TrimSpace(def.Name) == "" {
		return "", fmt.Errorf("theme file %q: missing required \"name\" field", path)
	}

	RegisterTheme(def.Name, def.Colors)
	return Theme(def.Name), nil
}

// LoadAndSetTheme loads a theme from a configuration file and activates it.
func LoadAndSetTheme(path string) error {
	theme, err := LoadThemeFile(path)
	if err != nil {
		return err
	}
	SetTheme(theme)
	return nil
}

// resolveColorSpec converts a human-friendly color specification into the
// concatenated ANSI escape codes it represents. See ThemeDefinition for the
// accepted syntax. Unknown tokens are skipped.
func resolveColorSpec(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	// A raw escape sequence is passed through verbatim.
	if strings.ContainsRune(spec, '\033') {
		return spec
	}

	var b strings.Builder
	for _, token := range strings.Fields(spec) {
		token = strings.ToLower(token)
		if code, ok := colorMap[token]; ok {
			b.WriteString(code)
			continue
		}
		if n, ok := parse256Color(token); ok {
			fmt.Fprintf(&b, "\033[38;5;%dm", n)
			continue
		}
		// Unknown token: ignored so a typo never injects garbage escapes.
	}
	return b.String()
}

// parse256Color parses a "256:N" token into its palette index (0-255).
func parse256Color(token string) (int, bool) {
	const prefix = "256:"
	if !strings.HasPrefix(token, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(token[len(prefix):])
	if err != nil || n < 0 || n > 255 {
		return 0, false
	}
	return n, true
}

// themeColors maps semantic color names to ANSI codes per theme. Registered
// custom themes take precedence; semantics they do not define fall back to the
// default palette so a partial custom theme remains usable.
func themeColors(semantic string) string {
	if colors, ok := customThemes[currentTheme]; ok {
		if code, defined := colors[semantic]; defined {
			return code
		}
		return defaultThemeColors(semantic)
	}

	switch currentTheme {
	case ThemeLight:
		switch semantic {
		case "success":
			return sgrBold + sgrGreen
		case "error":
			return sgrBold + sgrRed
		case "warning":
			return sgrBold + "\033[38;5;130m"
		case "info":
			return sgrBold + sgrBlue
		case "dim":
			return "\033[38;5;240m"
		case "bold":
			return sgrBold
		default:
			return ""
		}
	case ThemeDark:
		switch semantic {
		case "success":
			return "\033[38;5;46m"
		case "error":
			return "\033[38;5;196m"
		case "warning":
			return "\033[38;5;226m"
		case "info":
			return "\033[38;5;51m"
		case "dim":
			return "\033[38;5;244m"
		case "bold":
			return sgrBold
		default:
			return ""
		}
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
		return defaultThemeColors(semantic)
	}
}

// defaultThemeColors returns the ANSI code for a semantic slot in the default
// palette.
func defaultThemeColors(semantic string) string {
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
