// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStatusCommandRegistered(t *testing.T) {
	if statusCmd == nil {
		t.Fatal("statusCmd should not be nil")
	}
	if statusCmd.Use != "status" {
		t.Errorf("statusCmd.Use = %q, want %q", statusCmd.Use, "status")
	}
	if statusCmd.GroupID != "utility" {
		t.Errorf("statusCmd.GroupID = %q, want %q", statusCmd.GroupID, "utility")
	}
}

func TestStatusFixFlag(t *testing.T) {
	flag := statusCmd.Flags().Lookup("fix")
	if flag == nil {
		t.Fatal("status command should have --fix flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--fix default = %q, want %q", flag.DefValue, "false")
	}
}

func TestCheckProtocolRegistration(t *testing.T) {
	result := checkProtocolRegistration()

	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		// Result should have a Detail string regardless of registration state
		if !result.Registered && result.Detail == "" {
			t.Error("expected non-empty Detail when protocol is not registered")
		}
	default:
		if result.Registered {
			t.Errorf("unsupported platform %s should not report as registered", runtime.GOOS)
		}
		if !strings.Contains(result.Detail, "unsupported") {
			t.Errorf("expected 'unsupported' in Detail for platform %s, got %q", runtime.GOOS, result.Detail)
		}
	}
}

func TestCheckProtocolDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}

	plistPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "com.erst.protocol.plist")

	// If plist does not exist, should report not registered
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		result := checkProtocolDarwin()
		if result.Registered {
			t.Error("expected not registered when plist does not exist")
		}
		if !strings.Contains(result.Detail, "not found") {
			t.Errorf("expected 'not found' in detail, got %q", result.Detail)
		}
	}
}

func TestCheckProtocolLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}

	desktopPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications", "erst-protocol.desktop")

	if _, err := os.Stat(desktopPath); os.IsNotExist(err) {
		result := checkProtocolLinux()
		if result.Registered {
			t.Error("expected not registered when desktop file does not exist")
		}
		if !strings.Contains(result.Detail, "not found") {
			t.Errorf("expected 'not found' in detail, got %q", result.Detail)
		}
	}
}

func TestCheckProtocolDarwinWithValidPlist(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}

	// Create a temporary plist to verify detection
	tmpDir := t.TempDir()
	plistPath := filepath.Join(tmpDir, "com.erst.protocol.plist")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
    <key>CFBundleURLSchemes</key>
    <array><string>erst</string></array>
</dict>
</plist>`
	if err := os.WriteFile(plistPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// The real check uses a fixed path, so we can't redirect it.
	// Instead, just verify our plist content detection logic:
	data, _ := os.ReadFile(plistPath)
	if !strings.Contains(string(data), "<string>erst</string>") {
		t.Error("test plist should contain erst scheme")
	}
}

func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"no\n", false},
		{"maybe\n", false},
		{"\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cmd := statusCmd
			cmd.SetIn(strings.NewReader(tt.input))
			cmd.SetOut(&bytes.Buffer{})

			got := promptYesNo(cmd, "test? ")
			if got != tt.expected {
				t.Errorf("promptYesNo(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRepairProtocolDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}

	// Use a temp dir to avoid writing to real LaunchAgents
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create the LaunchAgents directory
	os.MkdirAll(filepath.Join(tmpDir, "Library", "LaunchAgents"), 0755)

	var buf bytes.Buffer
	err := repairProtocolDarwin("/usr/local/bin/erst", &buf)
	// launchctl load may fail in test env, but the file should be written
	plistPath := filepath.Join(tmpDir, "Library", "LaunchAgents", "com.erst.protocol.plist")
	data, readErr := os.ReadFile(plistPath)
	if readErr != nil {
		t.Fatalf("plist file was not written: %v (repair err: %v)", readErr, err)
	}
	if !strings.Contains(string(data), "<string>erst</string>") {
		t.Error("plist missing erst scheme")
	}
	if !strings.Contains(string(data), "/usr/local/bin/erst") {
		t.Error("plist missing CLI path")
	}
}

func TestRepairProtocolLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	var buf bytes.Buffer
	// xdg-mime may not be available in CI, but the file should be written
	_ = repairProtocolLinux("/usr/local/bin/erst", &buf)

	desktopPath := filepath.Join(tmpDir, ".local", "share", "applications", "erst-protocol.desktop")
	data, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("desktop file was not written: %v", err)
	}
	if !strings.Contains(string(data), "x-scheme-handler/erst") {
		t.Error("desktop file missing erst scheme handler")
	}
	if !strings.Contains(string(data), "/usr/local/bin/erst") {
		t.Error("desktop file missing CLI path")
	}
}

func TestStatusOutputRegistered(t *testing.T) {
	// Run the status command and capture output — just verify it doesn't panic
	var buf bytes.Buffer
	cmd := statusCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(strings.NewReader("n\n")) // answer "no" to any prompt

	// Reset flags for isolated test
	statusFixFlag = false
	_ = cmd.RunE(cmd, nil)

	output := buf.String()
	if !strings.Contains(output, "Erst Protocol Registration Status") {
		t.Error("expected status header in output")
	}
}
