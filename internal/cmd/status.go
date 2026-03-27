// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var statusFixFlag bool

var statusCmd = &cobra.Command{
	Use:     "status",
	GroupID: "utility",
	Short:   "Check protocol registration and system health",
	Long: `Inspect the health of the erst:// protocol handler registration.

This command verifies that the custom URI scheme (erst://) is properly
registered with the operating system so that deep links work correctly.

On failure it can interactively offer to repair the registration:
  - Windows: rewrites HKCU\Software\Classes\erst registry keys
  - macOS:   rewrites ~/Library/LaunchAgents/com.erst.protocol.plist
  - Linux:   rewrites ~/.local/share/applications/erst-protocol.desktop

Use --fix to skip the interactive prompt and repair automatically.`,
	Example: `  # Check protocol registration status
  erst status

  # Automatically repair without prompting
  erst status --fix`,
	Args: cobra.NoArgs,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Erst Protocol Registration Status")
	fmt.Fprintln(out, "==================================")
	fmt.Fprintln(out)

	result := checkProtocolRegistration()

	if result.Registered {
		fmt.Fprintf(out, "\033[32m[OK]\033[0m Protocol handler (erst://) is registered\n")
		if result.Detail != "" {
			fmt.Fprintf(out, "  Path: %s\n", result.Detail)
		}
		fmt.Fprintln(out)
		fmt.Fprintln(out, "\033[32mAll checks passed.\033[0m")
		return nil
	}

	// Registration is broken
	fmt.Fprintf(out, "\033[31m[FAIL]\033[0m Protocol handler (erst://) is not registered\n")
	if result.Detail != "" {
		fmt.Fprintf(out, "  \033[33m→ %s\033[0m\n", result.Detail)
	}
	fmt.Fprintln(out)

	// Determine whether to attempt repair
	shouldFix := statusFixFlag
	if !shouldFix && isInteractiveTTY(cmd) {
		shouldFix = promptYesNo(cmd, "Would you like ERST to repair the protocol registration? [y/n]: ")
	}

	if !shouldFix {
		fmt.Fprintln(out, "Skipping repair. Run 'erst status --fix' to repair automatically.")
		return nil
	}

	// Attempt repair
	fmt.Fprintln(out, "Repairing protocol registration...")
	if err := repairProtocolRegistration(out); err != nil {
		fmt.Fprintf(out, "\033[31m[FAIL]\033[0m Repair failed: %v\n", err)
		return fmt.Errorf("protocol repair failed: %w", err)
	}

	// Verify the fix
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Verifying repair...")
	verify := checkProtocolRegistration()
	if verify.Registered {
		fmt.Fprintf(out, "\033[32m[OK]\033[0m Protocol registration repaired successfully\n")
		return nil
	}

	fmt.Fprintf(out, "\033[31m[FAIL]\033[0m Verification failed — registration still broken\n")
	if verify.Detail != "" {
		fmt.Fprintf(out, "  \033[33m→ %s\033[0m\n", verify.Detail)
	}
	return fmt.Errorf("protocol registration repair could not be verified")
}

// ProtocolCheckResult holds the outcome of a protocol registration check.
type ProtocolCheckResult struct {
	Registered bool
	Detail     string
}

// checkProtocolRegistration inspects OS-specific artefacts to determine whether
// the erst:// URI scheme handler is registered.
func checkProtocolRegistration() ProtocolCheckResult {
	switch runtime.GOOS {
	case "windows":
		return checkProtocolWindows()
	case "darwin":
		return checkProtocolDarwin()
	case "linux":
		return checkProtocolLinux()
	default:
		return ProtocolCheckResult{Detail: fmt.Sprintf("unsupported platform: %s", runtime.GOOS)}
	}
}

func checkProtocolWindows() ProtocolCheckResult {
	regPath := `HKEY_CURRENT_USER\Software\Classes\erst`
	out, err := exec.Command("reg", "query", regPath).CombinedOutput()
	if err != nil {
		return ProtocolCheckResult{Detail: "Registry key HKCU\\Software\\Classes\\erst not found"}
	}
	if !strings.Contains(string(out), "URL Protocol") {
		return ProtocolCheckResult{Detail: "Registry key exists but missing URL Protocol value"}
	}
	return ProtocolCheckResult{Registered: true, Detail: regPath}
}

func checkProtocolDarwin() ProtocolCheckResult {
	plistPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "com.erst.protocol.plist")
	info, err := os.Stat(plistPath)
	if err != nil {
		return ProtocolCheckResult{Detail: fmt.Sprintf("Plist not found at %s", plistPath)}
	}
	if info.Size() == 0 {
		return ProtocolCheckResult{Detail: fmt.Sprintf("Plist is empty at %s", plistPath)}
	}

	// Verify the plist contains the expected protocol scheme
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return ProtocolCheckResult{Detail: fmt.Sprintf("Cannot read plist: %v", err)}
	}
	if !strings.Contains(string(data), "<string>erst</string>") {
		return ProtocolCheckResult{Detail: "Plist exists but does not contain erst:// scheme"}
	}

	return ProtocolCheckResult{Registered: true, Detail: plistPath}
}

func checkProtocolLinux() ProtocolCheckResult {
	desktopPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications", "erst-protocol.desktop")
	info, err := os.Stat(desktopPath)
	if err != nil {
		return ProtocolCheckResult{Detail: fmt.Sprintf("Desktop file not found at %s", desktopPath)}
	}
	if info.Size() == 0 {
		return ProtocolCheckResult{Detail: fmt.Sprintf("Desktop file is empty at %s", desktopPath)}
	}

	data, err := os.ReadFile(desktopPath)
	if err != nil {
		return ProtocolCheckResult{Detail: fmt.Sprintf("Cannot read desktop file: %v", err)}
	}
	if !strings.Contains(string(data), "x-scheme-handler/erst") {
		return ProtocolCheckResult{Detail: "Desktop file exists but does not contain erst scheme handler"}
	}

	return ProtocolCheckResult{Registered: true, Detail: desktopPath}
}

// repairProtocolRegistration writes the correct protocol handler artefacts for
// the current platform. On macOS/Linux this may require elevated permissions
// for certain paths, so we attempt the write and surface any permission errors.
func repairProtocolRegistration(out io.Writer) error {
	cliPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return repairProtocolWindows(cliPath, out)
	case "darwin":
		return repairProtocolDarwin(cliPath, out)
	case "linux":
		return repairProtocolLinux(cliPath, out)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func repairProtocolWindows(cliPath string, out io.Writer) error {
	regPath := `HKEY_CURRENT_USER\Software\Classes\erst`

	commands := []struct {
		desc string
		args []string
	}{
		{"Setting URL protocol type", []string{"reg", "add", regPath, "/ve", "/d", "URL:ERST Protocol", "/f"}},
		{"Setting URL Protocol value", []string{"reg", "add", regPath, "/v", "URL Protocol", "/d", "", "/f"}},
		{"Setting shell open command", []string{"reg", "add", regPath + `\shell\open\command`, "/ve", "/d", fmt.Sprintf(`"%s" protocol-handler "%%1"`, cliPath), "/f"}},
	}

	for _, c := range commands {
		fmt.Fprintf(out, "  %s...\n", c.desc)
		cmd := exec.Command(c.args[0], c.args[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w\n%s", c.desc, err, string(output))
		}
	}

	return nil
}

func repairProtocolDarwin(cliPath string, out io.Writer) error {
	plistDir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, "com.erst.protocol.plist")

	// Ensure the LaunchAgents directory exists
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return fmt.Errorf("cannot create LaunchAgents directory: %w", err)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.erst.protocol</string>
    <key>CFBundleURLTypes</key>
    <array>
        <dict>
            <key>CFBundleURLName</key>
            <string>ERST Protocol</string>
            <key>CFBundleURLSchemes</key>
            <array>
                <string>erst</string>
            </array>
        </dict>
    </array>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>protocol-handler</string>
    </array>
    <key>StandardInPath</key>
    <string>/dev/null</string>
    <key>StandardOutPath</key>
    <string>/tmp/erst-protocol.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/erst-protocol-error.log</string>
</dict>
</plist>`, cliPath)

	// Unload existing plist if present (ignore errors — may not be loaded)
	fmt.Fprintln(out, "  Unloading existing plist (if any)...")
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	fmt.Fprintln(out, "  Writing plist file...")
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("cannot write plist: %w", err)
	}

	fmt.Fprintln(out, "  Loading plist via launchctl...")
	if output, err := exec.Command("launchctl", "load", plistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load failed: %w\n%s", err, string(output))
	}

	return nil
}

func repairProtocolLinux(cliPath string, out io.Writer) error {
	appsDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")
	desktopPath := filepath.Join(appsDir, "erst-protocol.desktop")

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Version=1.0
Type=Application
Name=ERST Protocol Handler
Exec=%s protocol-handler %%u
MimeType=x-scheme-handler/erst;
NoDisplay=true
Terminal=false`, cliPath)

	fmt.Fprintln(out, "  Ensuring applications directory exists...")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return fmt.Errorf("cannot create applications directory: %w", err)
	}

	fmt.Fprintln(out, "  Writing desktop file...")
	if err := os.WriteFile(desktopPath, []byte(desktopContent), 0644); err != nil {
		return fmt.Errorf("cannot write desktop file: %w", err)
	}

	fmt.Fprintln(out, "  Registering MIME type...")
	if output, err := exec.Command("xdg-mime", "default", "erst-protocol.desktop", "x-scheme-handler/erst").CombinedOutput(); err != nil {
		return fmt.Errorf("xdg-mime failed: %w\n%s", err, string(output))
	}

	fmt.Fprintln(out, "  Updating desktop database...")
	_ = exec.Command("update-desktop-database", appsDir).Run()

	return nil
}

// isInteractiveTTY returns true when stdin is attached to an interactive terminal.
func isInteractiveTTY(cmd *cobra.Command) bool {
	inFile, ok := cmd.InOrStdin().(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(inFile.Fd())
}

// promptYesNo prints the prompt and waits for a y/n answer. Returns true for "y" or "yes".
func promptYesNo(cmd *cobra.Command, prompt string) bool {
	reader := bufio.NewReader(cmd.InOrStdin())
	out := cmd.OutOrStdout()

	fmt.Fprint(out, prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer := strings.TrimSpace(strings.ToLower(input))
	return answer == "y" || answer == "yes"
}

func init() {
	statusCmd.Flags().BoolVar(&statusFixFlag, "fix", false, "Automatically repair broken protocol registration without prompting")
	rootCmd.AddCommand(statusCmd)
}
