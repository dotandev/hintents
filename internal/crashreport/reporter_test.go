// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package crashreport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helpers ----------------------------------------------------------------

func newTestServer(t *testing.T, statusCode int, handler func(r *http.Request, body Report)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler != nil {
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var report Report
			require.NoError(t, json.Unmarshal(raw, &report))
			handler(r, report)
		}
		w.WriteHeader(statusCode)
	}))
}

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// ---- IsEnabled --------------------------------------------------------------

func TestIsEnabled_DisabledByDefault(t *testing.T) {
	os.Unsetenv(envOptIn)
	r := New(Config{Enabled: false})
	assert.False(t, r.IsEnabled())
}

func TestIsEnabled_EnabledViaConfig(t *testing.T) {
	os.Unsetenv(envOptIn)
	r := New(Config{Enabled: true})
	assert.True(t, r.IsEnabled())
}

func TestIsEnabled_EnvVarTrueOverridesConfig(t *testing.T) {
	setEnv(t, envOptIn, "true")
	r := New(Config{Enabled: false})
	assert.True(t, r.IsEnabled())
}

func TestIsEnabled_EnvVar1(t *testing.T) {
	setEnv(t, envOptIn, "1")
	r := New(Config{Enabled: false})
	assert.True(t, r.IsEnabled())
}

func TestIsEnabled_EnvVarFalseOverridesConfig(t *testing.T) {
	setEnv(t, envOptIn, "false")
	r := New(Config{Enabled: true})
	assert.False(t, r.IsEnabled())
}

func TestIsEnabled_EnvVar0(t *testing.T) {
	setEnv(t, envOptIn, "0")
	r := New(Config{Enabled: true})
	assert.False(t, r.IsEnabled())
}

// ---- New / config -----------------------------------------------------------

func TestNew_DefaultEndpoint(t *testing.T) {
	os.Unsetenv(envEndpoint)
	r := New(Config{})
	assert.Equal(t, DefaultEndpoint, r.cfg.Endpoint)
}

func TestNew_CustomEndpointFromConfig(t *testing.T) {
	os.Unsetenv(envEndpoint)
	r := New(Config{Endpoint: "https://example.com/crash"})
	assert.Equal(t, "https://example.com/crash", r.cfg.Endpoint)
}

func TestNew_EndpointEnvVarOverridesConfig(t *testing.T) {
	setEnv(t, envEndpoint, "https://env-override.example.com/crash")
	r := New(Config{Endpoint: "https://config.example.com/crash"})
	assert.Equal(t, "https://env-override.example.com/crash", r.cfg.Endpoint)
}

// ---- Send -------------------------------------------------------------------

func TestSend_NoOpWhenDisabled(t *testing.T) {
	os.Unsetenv(envOptIn)
	srv := newTestServer(t, http.StatusOK, func(r *http.Request, body Report) {
		t.Fatal("server should not be called when crash reporting is disabled")
	})
	defer srv.Close()

	reporter := New(Config{Enabled: false, Endpoint: srv.URL})
	err := reporter.Send(context.Background(), errors.New("test"), nil, "erst debug")
	assert.NoError(t, err)
}

func TestSend_PostsPayloadWhenEnabled(t *testing.T) {
	setEnv(t, envOptIn, "true")

	var received Report
	srv := newTestServer(t, http.StatusOK, func(r *http.Request, body Report) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		received = body
	})
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL, Version: "1.2.3", CommitSHA: "abc123"})
	err := reporter.Send(context.Background(), errors.New("boom"), []byte("goroutine 1\n..."), "erst debug")

	require.NoError(t, err)
	assert.Equal(t, "boom", received.ErrorMessage)
	assert.Equal(t, "goroutine 1\n...", received.StackTrace)
	assert.Equal(t, "erst debug", received.Command)
	assert.Equal(t, "1.2.3", received.Version)
	assert.Equal(t, "abc123", received.CommitSHA)
	assert.Equal(t, runtime.GOOS, received.OS)
	assert.Equal(t, runtime.GOARCH, received.Arch)
	assert.NotEmpty(t, received.CrashTime)
}

func TestSend_UserAgentIncludesVersion(t *testing.T) {
	setEnv(t, envOptIn, "true")

	var gotUA string
	srv := newTestServer(t, http.StatusOK, func(r *http.Request, body Report) {
		gotUA = r.Header.Get("User-Agent")
	})
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL, Version: "0.9.0"})
	require.NoError(t, reporter.Send(context.Background(), errors.New("x"), nil, ""))
	assert.Equal(t, "erst/0.9.0", gotUA)
}

func TestSend_NilErrorSendsEmptyMessage(t *testing.T) {
	setEnv(t, envOptIn, "true")

	var received Report
	srv := newTestServer(t, http.StatusOK, func(_ *http.Request, body Report) {
		received = body
	})
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL})
	require.NoError(t, reporter.Send(context.Background(), nil, nil, ""))
	assert.Equal(t, "", received.ErrorMessage)
}

func TestSend_ReturnsErrorOnHTTP4xx(t *testing.T) {
	setEnv(t, envOptIn, "true")

	srv := newTestServer(t, http.StatusBadRequest, nil)
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL})
	err := reporter.Send(context.Background(), errors.New("err"), nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestSend_ReturnsErrorOnHTTP5xx(t *testing.T) {
	setEnv(t, envOptIn, "true")

	srv := newTestServer(t, http.StatusInternalServerError, nil)
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL})
	err := reporter.Send(context.Background(), errors.New("err"), nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSend_ReturnsErrorOnUnreachableEndpoint(t *testing.T) {
	setEnv(t, envOptIn, "true")

	reporter := New(Config{Enabled: true, Endpoint: "http://localhost:0/unreachable"})
	err := reporter.Send(context.Background(), errors.New("err"), nil, "")
	require.Error(t, err)
}

func TestSend_NilStackOmittedFromPayload(t *testing.T) {
	setEnv(t, envOptIn, "true")

	var received Report
	srv := newTestServer(t, http.StatusOK, func(_ *http.Request, body Report) {
		received = body
	})
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL})
	require.NoError(t, reporter.Send(context.Background(), errors.New("err"), nil, ""))
	assert.Equal(t, "", received.StackTrace)
}

// ---- buildReport ------------------------------------------------------------

func TestBuildReport_FieldsPopulated(t *testing.T) {
	os.Unsetenv(envOptIn)
	reporter := New(Config{Version: "2.0.0", CommitSHA: "deadbeef"})
	report := reporter.buildReport(errors.New("something failed"), []byte("stack"), "erst session list")

	assert.Equal(t, "2.0.0", report.Version)
	assert.Equal(t, "deadbeef", report.CommitSHA)
	assert.Equal(t, runtime.GOOS, report.OS)
	assert.Equal(t, runtime.GOARCH, report.Arch)
	assert.Equal(t, "something failed", report.ErrorMessage)
	assert.Equal(t, "stack", report.StackTrace)
	assert.Equal(t, "erst session list", report.Command)
	assert.True(t, strings.HasPrefix(report.CrashTime, "20"), "CrashTime should be a year prefix")
}

// ---- HandlePanic ------------------------------------------------------------

func TestHandlePanic_NoOpOnNoPanic(t *testing.T) {
	srv := newTestServer(t, http.StatusOK, func(r *http.Request, body Report) {
		t.Fatal("server should not be called when there is no panic")
	})
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL})

	// Should not call the server and should not re-panic.
	assert.NotPanics(t, func() {
		reporter.HandlePanic(context.Background(), "erst debug")
	})
}

func TestHandlePanic_ReportsAndRepanicsOnError(t *testing.T) {
	setEnv(t, envOptIn, "true")

	reported := false
	srv := newTestServer(t, http.StatusOK, func(_ *http.Request, body Report) {
		reported = true
		assert.Equal(t, "fatal error occurred", body.ErrorMessage)
	})
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL})

	assert.Panics(t, func() {
		defer reporter.HandlePanic(context.Background(), "erst debug")
		panic(errors.New("fatal error occurred"))
	})

	assert.True(t, reported, "crash report should have been sent")
}

func TestHandlePanic_ReportsStringPanic(t *testing.T) {
	setEnv(t, envOptIn, "true")

	var gotMessage string
	srv := newTestServer(t, http.StatusOK, func(_ *http.Request, body Report) {
		gotMessage = body.ErrorMessage
	})
	defer srv.Close()

	reporter := New(Config{Enabled: true, Endpoint: srv.URL})

	assert.Panics(t, func() {
		defer reporter.HandlePanic(context.Background(), "")
		panic("string panic value")
	})

	assert.Equal(t, "string panic value", gotMessage)
}

// ---- Sentry DSN / env-var wiring --------------------------------------------

// TestNew_SentryDSNFromEnv verifies that ERST_SENTRY_DSN is picked up by New
// and stored in the config (Sentry init will fail with a fake DSN, which is
// acceptable — we only verify the field is set, not that the SDK initialised).
func TestNew_SentryDSNFromEnv(t *testing.T) {
	setEnv(t, envSentryDSN, "https://fakekey@o0.ingest.sentry.io/0")
	// Endpoint must be empty in config so the env-var DSN is the only sink.
	r := New(Config{Endpoint: "unused"})
	assert.Equal(t, "https://fakekey@o0.ingest.sentry.io/0", r.cfg.SentryDSN)
}

// TestNew_SentryDSNFromConfig verifies that a DSN supplied directly via Config
// is stored and does not fall back to DefaultEndpoint when it is the only sink.
func TestNew_SentryDSNFromConfig(t *testing.T) {
	os.Unsetenv(envSentryDSN)
	os.Unsetenv(envEndpoint)
	r := New(Config{SentryDSN: "https://fakekey@o0.ingest.sentry.io/1"})
	assert.Equal(t, "https://fakekey@o0.ingest.sentry.io/1", r.cfg.SentryDSN)
	// Endpoint should remain empty — DSN is present, no fallback needed.
	assert.Equal(t, "", r.cfg.Endpoint)
}

// TestNew_DefaultEndpointWhenNoSinks verifies that DefaultEndpoint is used
// when neither SentryDSN nor Endpoint is configured.
func TestNew_DefaultEndpointWhenNoSinks(t *testing.T) {
	os.Unsetenv(envSentryDSN)
	os.Unsetenv(envEndpoint)
	r := New(Config{})
	assert.Equal(t, DefaultEndpoint, r.cfg.Endpoint)
	assert.Equal(t, "", r.cfg.SentryDSN)
}

// TestSend_SentrySkippedWhenInactive verifies that a reporter with an invalid
// DSN (sentryActive=false) still successfully delivers to the custom endpoint.
func TestSend_SentrySkippedWhenInactive(t *testing.T) {
	setEnv(t, envOptIn, "true")

	received := false
	srv := newTestServer(t, http.StatusOK, func(_ *http.Request, _ Report) {
		received = true
	})
	defer srv.Close()

	// Use a bad DSN so sentryActive stays false, but provide a working endpoint.
	r := New(Config{
		Enabled:   true,
		SentryDSN: "not-a-dsn",
		Endpoint:  srv.URL,
	})
	assert.False(t, r.sentryActive)

	err := r.Send(context.Background(), errors.New("test"), nil, "")
	require.NoError(t, err)
	assert.True(t, received, "custom endpoint should have been called")
}

// TestIsEnabled_EnvVarYes verifies the "yes" token is accepted.
func TestIsEnabled_EnvVarYes(t *testing.T) {
	setEnv(t, envOptIn, "yes")
	r := New(Config{Enabled: false})
	assert.True(t, r.IsEnabled())
}

// TestIsEnabled_EnvVarNo verifies the "no" token is accepted.
func TestIsEnabled_EnvVarNo(t *testing.T) {
	setEnv(t, envOptIn, "no")
	r := New(Config{Enabled: true})
	assert.False(t, r.IsEnabled())
}
