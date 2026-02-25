// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

// Package crashreport provides opt-in anonymous crash reporting for the Erst CLI.
//
// Two independent sinks are supported and may be used together:
//
//   - Sentry: supply a DSN via SentryDSN in Config or the ERST_SENTRY_DSN
//     environment variable.  The official Sentry Go SDK is used.
//
//   - Custom endpoint: supply an HTTPS URL via Endpoint in Config or the
//     ERST_CRASH_ENDPOINT environment variable.  A JSON Report is POSTed.
//
// Both sinks are disabled by default.  Users must explicitly opt in via the
// config file (crash_reporting = true) or the ERST_CRASH_REPORTING environment
// variable.  No personal data or transaction content is ever collected: only
// the error message, stack trace, OS/arch, Go version, and Erst version.
package crashreport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	// DefaultEndpoint is the default anonymous crash collection endpoint used
	// when Endpoint is empty and no SentryDSN is configured.
	DefaultEndpoint = "https://crash.erst.dev/v1/report"

	// defaultTimeout is the maximum time allowed for each outbound HTTP request.
	defaultTimeout = 5 * time.Second

	// envOptIn is the environment variable that enables crash reporting.
	envOptIn = "ERST_CRASH_REPORTING"

	// envEndpoint overrides the custom HTTP endpoint at runtime.
	envEndpoint = "ERST_CRASH_ENDPOINT"

	// envSentryDSN supplies the Sentry DSN at runtime.
	envSentryDSN = "ERST_SENTRY_DSN"
)

// Report is the JSON payload delivered to the custom endpoint.
// Fields are deliberately minimal to preserve user privacy.
type Report struct {
	// Version of the Erst binary that crashed.
	Version string `json:"version"`
	// CommitSHA is the VCS revision embedded at build time.
	CommitSHA string `json:"commit_sha,omitempty"`
	// OS and Arch are the GOOS / GOARCH values from the build environment.
	OS   string `json:"os"`
	Arch string `json:"arch"`
	// GoVersion is the Go toolchain used to compile the binary.
	GoVersion string `json:"go_version"`
	// CrashTime is the RFC 3339 timestamp of the crash.
	CrashTime string `json:"crash_time"`
	// ErrorMessage is the top-level error string (no user data).
	ErrorMessage string `json:"error_message"`
	// StackTrace is the goroutine dump captured at panic time.
	StackTrace string `json:"stack_trace,omitempty"`
	// Command is the cobra command path that was executing (e.g. "erst debug").
	Command string `json:"command,omitempty"`
}

// Config controls crash reporter behaviour.
type Config struct {
	// Enabled must be true for any report to be sent.
	Enabled bool
	// SentryDSN is the Sentry Data Source Name.  When non-empty, crashes are
	// forwarded to Sentry using the official Go SDK.  The ERST_SENTRY_DSN
	// environment variable overrides this value at runtime.
	SentryDSN string
	// Endpoint is the URL that accepts POST application/json crash reports.
	// When empty and SentryDSN is also empty, DefaultEndpoint is used.
	// The ERST_CRASH_ENDPOINT environment variable overrides this value at
	// runtime.
	Endpoint string
	// Version and CommitSHA are injected from build-time ldflags.
	Version   string
	CommitSHA string
}

// Reporter dispatches crash reports to all configured sinks.
type Reporter struct {
	cfg          Config
	client       *http.Client
	sentryActive bool
}

// New creates a Reporter from cfg, initialising Sentry if a DSN is available.
//
// Environment variable precedence (highest to lowest):
//
//	ERST_SENTRY_DSN        overrides cfg.SentryDSN
//	ERST_CRASH_ENDPOINT    overrides cfg.Endpoint
//	ERST_CRASH_REPORTING   overrides cfg.Enabled (checked at send time)
//
// When both SentryDSN and Endpoint are empty after env-var resolution,
// Endpoint falls back to DefaultEndpoint so that an operator who sets only
// ERST_CRASH_REPORTING=true still gets a working reporter.
func New(cfg Config) *Reporter {
	if dsn := os.Getenv(envSentryDSN); dsn != "" {
		cfg.SentryDSN = dsn
	}
	if ep := os.Getenv(envEndpoint); ep != "" {
		cfg.Endpoint = ep
	}
	if cfg.SentryDSN == "" && cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}

	r := &Reporter{
		cfg: cfg,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	if cfg.SentryDSN != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:     cfg.SentryDSN,
			Release: cfg.Version,
		}); err == nil {
			r.sentryActive = true
		}
	}

	return r
}

// IsEnabled returns true when crash reporting is active.
// The ERST_CRASH_REPORTING environment variable takes precedence over the
// Enabled field, allowing users to opt in or out without editing config files.
func (r *Reporter) IsEnabled() bool {
	switch os.Getenv(envOptIn) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	}
	return r.cfg.Enabled
}

// Send constructs a Report from err and stack, then dispatches it to every
// active sink (Sentry and/or custom endpoint).
//
// It returns without error if reporting is disabled.  Errors from individual
// sinks are joined and returned, but callers on a crash path should treat them
// as informational — the process is already exiting.
func (r *Reporter) Send(ctx context.Context, err error, stack []byte, command string) error {
	if !r.IsEnabled() {
		return nil
	}

	report := r.buildReport(err, stack, command)

	var errs []error

	if r.sentryActive {
		if sendErr := r.sendToSentry(report); sendErr != nil {
			errs = append(errs, sendErr)
		}
	}

	if r.cfg.Endpoint != "" {
		if sendErr := r.sendToEndpoint(ctx, report); sendErr != nil {
			errs = append(errs, sendErr)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("crashreport: %v", errs)
	}
	return nil
}

// sendToSentry forwards a report to the configured Sentry project.
func (r *Reporter) sendToSentry(report Report) error {
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("os", report.OS)
		scope.SetTag("arch", report.Arch)
		scope.SetTag("go_version", report.GoVersion)
		scope.SetTag("command", report.Command)
		scope.SetExtra("stack_trace", report.StackTrace)
		scope.SetExtra("commit_sha", report.CommitSHA)

		sentry.CaptureMessage(report.ErrorMessage)
	})
	sentry.Flush(defaultTimeout)
	return nil
}

// sendToEndpoint POSTs a JSON-encoded report to the custom HTTP endpoint.
func (r *Reporter) sendToEndpoint(ctx context.Context, report Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "erst/"+r.cfg.Version)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// buildReport constructs the Report value from the current process metadata.
func (r *Reporter) buildReport(err error, stack []byte, command string) Report {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	goVersion := "unknown"
	if bi, ok := debug.ReadBuildInfo(); ok {
		goVersion = bi.GoVersion
	}

	return Report{
		Version:      r.cfg.Version,
		CommitSHA:    r.cfg.CommitSHA,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    goVersion,
		CrashTime:    time.Now().UTC().Format(time.RFC3339),
		ErrorMessage: errMsg,
		StackTrace:   string(stack),
		Command:      command,
	}
}

// HandlePanic is intended to be deferred at the top of main or Execute.
// If a panic is in flight it captures the stack, sends a report (best-effort),
// then re-panics so the runtime still terminates with a non-zero exit code.
func (r *Reporter) HandlePanic(ctx context.Context, command string) {
	v := recover()
	if v == nil {
		return
	}

	stack := debug.Stack()

	var panicErr error
	switch e := v.(type) {
	case error:
		panicErr = e
	default:
		panicErr = fmt.Errorf("%v", e)
	}

	// Best-effort: ignore send errors — we are already in a fatal path.
	_ = r.Send(ctx, panicErr, stack, command)

	// Re-panic so Go's runtime prints the stack and exits non-zero.
	panic(v)
}
