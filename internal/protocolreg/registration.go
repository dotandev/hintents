// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package protocolreg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	Scheme             = "erst"
	windowsRegistryKey = `HKEY_CURRENT_USER\Software\Classes\erst`
	linuxDesktopFile   = "erst-protocol.desktop"
	linuxMimeType      = "x-scheme-handler/erst"
	macOSAppName       = "Erst Protocol.app"
	macOSBundleID      = "dev.dotan.erst.protocol"
	macOSExecutable    = "erst-protocol-handler"
)

type Registrar struct {
	executablePath string
	homeDir        string
}

type VerificationReport struct {
	Platform string
	Scheme   string
	Checks   []string
	Issues   []string
}

func NewRegistrar() (*Registrar, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}

	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute executable path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home directory: %w", err)
	}

	return &Registrar{
		executablePath: executablePath,
		homeDir:        homeDir,
	}, nil
}

func (r *Registrar) Register() error {
	switch runtime.GOOS {
	case "windows":
		return r.registerWindows()
	case "darwin":
		return r.registerDarwin()
	case "linux":
		return r.registerLinux()
	default:
		return fmt.Errorf("protocol registration is not supported on %s", runtime.GOOS)
	}
}

func (r *Registrar) Unregister() error {
	switch runtime.GOOS {
	case "windows":
		return r.unregisterWindows()
	case "darwin":
		return r.unregisterDarwin()
	case "linux":
		return r.unregisterLinux()
	default:
		return fmt.Errorf("protocol registration is not supported on %s", runtime.GOOS)
	}
}

func (r *Registrar) IsRegistered() bool {
	_, err := r.Verify()
	return err == nil
}

func (r *Registrar) Verify() (*VerificationReport, error) {
	report := &VerificationReport{
		Platform: runtime.GOOS,
		Scheme:   Scheme,
	}

	switch runtime.GOOS {
	case "windows":
		r.verifyWindows(report)
	case "darwin":
		r.verifyDarwin(report)
	case "linux":
		r.verifyLinux(report)
	default:
		report.Issues = append(report.Issues, fmt.Sprintf("protocol verification is not supported on %s", runtime.GOOS))
	}

	if len(report.Issues) > 0 {
		return report, verificationError(report.Issues)
	}

	return report, nil
}

func (r *Registrar) registerWindows() error {
	commands := [][]string{
		{"add", windowsRegistryKey, "/ve", "/d", "URL:ERST Protocol", "/f"},
		{"add", windowsRegistryKey, "/v", "URL Protocol", "/d", "", "/f"},
		{"add", windowsRegistryKey + `\shell\open\command`, "/ve", "/d", r.windowsOpenCommand(), "/f"},
	}

	for _, args := range commands {
		if _, err := runCommand("reg", args...); err != nil {
			return err
		}
	}

	return nil
}

func (r *Registrar) unregisterWindows() error {
	_, err := runCommand("reg", "delete", windowsRegistryKey, "/f")
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unable to find") {
		return nil
	}
	return err
}

func (r *Registrar) verifyWindows(report *VerificationReport) {
	registryOutput, err := runCommand("reg", "query", windowsRegistryKey)
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing registry key %s: %v", windowsRegistryKey, err))
		return
	}
	report.Checks = append(report.Checks, fmt.Sprintf("Found registry key %s", windowsRegistryKey))

	if !strings.Contains(registryOutput, "URL Protocol") {
		report.Issues = append(report.Issues, "missing URL Protocol registry value")
	} else {
		report.Checks = append(report.Checks, "Found URL Protocol registry value")
	}

	commandOutput, err := runCommand("reg", "query", windowsRegistryKey+`\shell\open\command`, "/ve")
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing shell open command: %v", err))
		return
	}

	if !strings.Contains(commandOutput, r.windowsOpenCommand()) {
		report.Issues = append(report.Issues, fmt.Sprintf("unexpected shell open command, want %q", r.windowsOpenCommand()))
	} else {
		report.Checks = append(report.Checks, fmt.Sprintf("Shell open command points to %s", r.executablePath))
	}
}

func (r *Registrar) registerLinux() error {
	applicationsDir := filepath.Dir(r.linuxDesktopPath())
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		return fmt.Errorf("create applications directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(r.linuxWrapperPath()), 0o755); err != nil {
		return fmt.Errorf("create protocol helper directory: %w", err)
	}

	if err := os.WriteFile(r.linuxWrapperPath(), []byte(r.unixHandlerScript()), 0o755); err != nil {
		return fmt.Errorf("write protocol helper script: %w", err)
	}

	if err := os.WriteFile(r.linuxDesktopPath(), []byte(r.linuxDesktopEntry()), 0o644); err != nil {
		return fmt.Errorf("write desktop file: %w", err)
	}

	if _, err := runCommand("xdg-mime", "default", linuxDesktopFile, linuxMimeType); err != nil {
		return err
	}

	if hasCommand("update-desktop-database") {
		if _, err := runCommand("update-desktop-database", filepath.Dir(r.linuxDesktopPath())); err != nil {
			return err
		}
	}

	return nil
}

func (r *Registrar) unregisterLinux() error {
	var joined error

	if err := os.Remove(r.linuxDesktopPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		joined = errors.Join(joined, fmt.Errorf("remove desktop file: %w", err))
	}
	if err := os.Remove(r.linuxWrapperPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		joined = errors.Join(joined, fmt.Errorf("remove protocol helper: %w", err))
	}

	return joined
}

func (r *Registrar) verifyLinux(report *VerificationReport) {
	desktopBytes, err := os.ReadFile(r.linuxDesktopPath())
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing desktop file %s: %v", r.linuxDesktopPath(), err))
		return
	}

	desktopContent := string(desktopBytes)
	report.Checks = append(report.Checks, fmt.Sprintf("Found desktop file %s", r.linuxDesktopPath()))

	if !strings.Contains(desktopContent, "MimeType="+linuxMimeType+";") {
		report.Issues = append(report.Issues, "desktop file is missing the erst MIME handler declaration")
	} else {
		report.Checks = append(report.Checks, "Desktop file declares x-scheme-handler/erst")
	}

	if !strings.Contains(desktopContent, "Exec="+r.linuxWrapperPath()+" %u") {
		report.Issues = append(report.Issues, fmt.Sprintf("desktop file Exec entry does not point to %s", r.linuxWrapperPath()))
	} else {
		report.Checks = append(report.Checks, fmt.Sprintf("Desktop file Exec entry points to %s", r.linuxWrapperPath()))
	}

	wrapperBytes, err := os.ReadFile(r.linuxWrapperPath())
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing protocol helper script %s: %v", r.linuxWrapperPath(), err))
	} else {
		wrapperContent := string(wrapperBytes)
		if !strings.Contains(wrapperContent, r.executablePath) {
			report.Issues = append(report.Issues, fmt.Sprintf("protocol helper script does not reference %s", r.executablePath))
		} else {
			report.Checks = append(report.Checks, fmt.Sprintf("Protocol helper script launches %s", r.executablePath))
		}
	}

	defaultDesktop, err := runCommand("xdg-mime", "query", "default", linuxMimeType)
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("failed to query MIME handler: %v", err))
		return
	}

	if strings.TrimSpace(defaultDesktop) != linuxDesktopFile {
		report.Issues = append(report.Issues, fmt.Sprintf("xdg-mime reports %q instead of %q", strings.TrimSpace(defaultDesktop), linuxDesktopFile))
	} else {
		report.Checks = append(report.Checks, fmt.Sprintf("xdg-mime resolves %s to %s", linuxMimeType, linuxDesktopFile))
	}
}

func (r *Registrar) registerDarwin() error {
	if err := os.MkdirAll(filepath.Dir(r.macOSExecutablePath()), 0o755); err != nil {
		return fmt.Errorf("create macOS app bundle: %w", err)
	}

	if err := os.WriteFile(r.macOSExecutablePath(), []byte(r.unixHandlerScript()), 0o755); err != nil {
		return fmt.Errorf("write app bundle executable: %w", err)
	}

	if err := os.WriteFile(r.macOSPlistPath(), []byte(r.macOSInfoPlist()), 0o644); err != nil {
		return fmt.Errorf("write app bundle plist: %w", err)
	}

	if _, err := runCommand(macOSLSRegisterPath(), "-f", r.macOSAppPath()); err != nil {
		return err
	}

	return nil
}

func (r *Registrar) unregisterDarwin() error {
	var joined error

	if _, err := os.Stat(r.macOSAppPath()); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if _, err := runCommand(macOSLSRegisterPath(), "-u", r.macOSAppPath()); err != nil {
		joined = errors.Join(joined, err)
	}

	if err := os.RemoveAll(r.macOSAppPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		joined = errors.Join(joined, fmt.Errorf("remove macOS app bundle: %w", err))
	}

	return joined
}

func (r *Registrar) verifyDarwin(report *VerificationReport) {
	plistBytes, err := os.ReadFile(r.macOSPlistPath())
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing app bundle plist %s: %v", r.macOSPlistPath(), err))
		return
	}
	plistContent := string(plistBytes)
	report.Checks = append(report.Checks, fmt.Sprintf("Found app bundle plist %s", r.macOSPlistPath()))

	if _, err := os.Stat(r.macOSExecutablePath()); err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("missing app bundle executable %s: %v", r.macOSExecutablePath(), err))
	} else {
		report.Checks = append(report.Checks, fmt.Sprintf("Found app bundle executable %s", r.macOSExecutablePath()))
	}

	if !strings.Contains(plistContent, "<key>CFBundleURLSchemes</key>") || !strings.Contains(plistContent, "<string>"+Scheme+"</string>") {
		report.Issues = append(report.Issues, "Info.plist does not declare the erst URL scheme")
	} else {
		report.Checks = append(report.Checks, "Info.plist declares the erst URL scheme")
	}

	registrationDump, err := runCommand(macOSLSRegisterPath(), "-dump")
	if err != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("failed to inspect LaunchServices registration: %v", err))
		return
	}

	if !strings.Contains(registrationDump, r.macOSAppPath()) {
		report.Issues = append(report.Issues, fmt.Sprintf("LaunchServices dump does not contain %s", r.macOSAppPath()))
	} else {
		report.Checks = append(report.Checks, fmt.Sprintf("LaunchServices contains %s", r.macOSAppPath()))
	}
	if !strings.Contains(registrationDump, Scheme) {
		report.Issues = append(report.Issues, "LaunchServices dump does not mention the erst scheme")
	} else {
		report.Checks = append(report.Checks, "LaunchServices dump mentions the erst scheme")
	}

	executableBytes, err := os.ReadFile(r.macOSExecutablePath())
	if err == nil {
		executableContent := string(executableBytes)
		if !strings.Contains(executableContent, r.executablePath) {
			report.Issues = append(report.Issues, fmt.Sprintf("app bundle executable does not launch %s", r.executablePath))
		} else {
			report.Checks = append(report.Checks, fmt.Sprintf("App bundle executable launches %s", r.executablePath))
		}
	}
}

func (r *Registrar) windowsOpenCommand() string {
	return fmt.Sprintf(`"%s" protocol-handler "%%1"`, r.executablePath)
}

func (r *Registrar) linuxDesktopPath() string {
	return filepath.Join(r.homeDir, ".local", "share", "applications", linuxDesktopFile)
}

func (r *Registrar) linuxWrapperPath() string {
	return filepath.Join(r.homeDir, ".local", "share", "erst", macOSExecutable)
}

func (r *Registrar) linuxDesktopEntry() string {
	return strings.Join([]string{
		"[Desktop Entry]",
		"Version=1.0",
		"Type=Application",
		"Name=ERST Protocol Handler",
		"Exec=" + r.linuxWrapperPath() + " %u",
		"MimeType=" + linuxMimeType + ";",
		"NoDisplay=true",
		"Terminal=false",
		"",
	}, "\n")
}

func (r *Registrar) macOSAppPath() string {
	return filepath.Join(r.homeDir, "Applications", macOSAppName)
}

func (r *Registrar) macOSExecutablePath() string {
	return filepath.Join(r.macOSAppPath(), "Contents", "MacOS", macOSExecutable)
}

func (r *Registrar) macOSPlistPath() string {
	return filepath.Join(r.macOSAppPath(), "Contents", "Info.plist")
}

func (r *Registrar) macOSInfoPlist() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleExecutable</key>
    <string>%s</string>
    <key>CFBundleIdentifier</key>
    <string>%s</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>ERST Protocol Handler</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundleURLTypes</key>
    <array>
        <dict>
            <key>CFBundleURLName</key>
            <string>ERST Protocol</string>
            <key>CFBundleURLSchemes</key>
            <array>
                <string>%s</string>
            </array>
        </dict>
    </array>
</dict>
</plist>
`, macOSExecutable, macOSBundleID, Scheme)
}

func (r *Registrar) unixHandlerScript() string {
	return fmt.Sprintf("#!/bin/sh\nexec %s protocol-handler \"$1\"\n", shellQuote(r.executablePath))
}

func macOSLSRegisterPath() string {
	return "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return string(output), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, trimmed)
	}
	return string(output), nil
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func verificationError(issues []string) error {
	return fmt.Errorf("protocol registration verification failed: %s", strings.Join(issues, "; "))
}
