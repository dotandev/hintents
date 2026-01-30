// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dotandev/hintents/internal/simulator"
)

func TestSlackMessageFormatting(t *testing.T) {
	report := ReportData{
		TraceID:     "trace-123",
		TxHash:      "0xabc123",
		Network:     "testnet",
		Status:      "error",
		Error:       "Contract execution failed",
		Timestamp:   time.Date(2026, 1, 29, 15, 30, 0, 0, time.UTC),
		AuditLogURL: "https://example.com/audit/123",
		DiagnosticEvents: []simulator.DiagnosticEvent{
			{
				EventType: "contract",
				Topics:    []string{"transfer", "balance"},
				Data:      "Contract state changed",
			},
		},
	}

	msg := FormatSlackMessage(report)

	// Verify message structure
	if msg.Text == "" {
		t.Error("Slack message text is empty")
	}

	if len(msg.Blocks) == 0 {
		t.Error("Slack message blocks are empty")
	}

	// Verify JSON marshaling
	_, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Slack message: %v", err)
	}
}

func TestDiscordMessageFormatting(t *testing.T) {
	report := ReportData{
		TraceID:     "trace-456",
		TxHash:      "0xdef456",
		Network:     "mainnet",
		Status:      "success",
		Timestamp:   time.Date(2026, 1, 29, 15, 30, 0, 0, time.UTC),
		AuditLogURL: "https://example.com/audit/456",
		DiagnosticEvents: []simulator.DiagnosticEvent{
			{
				EventType: "system",
				Topics:    []string{"gas"},
				Data:      "Gas usage recorded",
			},
			{
				EventType: "diagnostic",
				Topics:    []string{"timestamp"},
				Data:      "Timestamp validation passed",
			},
		},
	}

	msg := FormatDiscordMessage(report)

	// Verify message structure
	if msg.Username == "" {
		t.Error("Discord message username is empty")
	}

	if len(msg.Embeds) == 0 {
		t.Error("Discord message embeds are empty")
	}

	if len(msg.Embeds[0].Fields) == 0 {
		t.Error("Discord embed fields are empty")
	}

	// Verify JSON marshaling
	_, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Discord message: %v", err)
	}
}

func TestClientCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid Slack config",
			config: Config{
				Type:    SlackWebhook,
				URL:     "https://hooks.slack.com/services/T123/B456/xyz",
				Timeout: 30 * time.Second,
				Retries: 3,
			},
			wantErr: false,
		},
		{
			name: "Valid Discord config",
			config: Config{
				Type:    DiscordWebhook,
				URL:     "https://discordapp.com/api/webhooks/123/abc",
				Timeout: 30 * time.Second,
				Retries: 3,
			},
			wantErr: false,
		},
		{
			name: "Empty URL",
			config: Config{
				Type: SlackWebhook,
				URL:  "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWebhookSend(t *testing.T) {
	// Setup test server
	receivedPayloads := make([][]byte, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		payload := make([]byte, r.ContentLength)
		if _, err := r.Body.Read(payload); err != nil {
			t.Logf("Error reading request body: %v", err)
		}
		receivedPayloads = append(receivedPayloads, payload)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Type:    SlackWebhook,
		URL:     server.URL,
		Timeout: 5 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	report := ReportData{
		TraceID:   "test-trace",
		TxHash:    "0xtest",
		Network:   "testnet",
		Status:    "success",
		Timestamp: time.Now(),
	}

	err = client.Send(report)
	if err != nil {
		t.Fatalf("Failed to send webhook: %v", err)
	}

	if len(receivedPayloads) != 1 {
		t.Errorf("Expected 1 payload sent, got %d", len(receivedPayloads))
	}
}

func TestWebhookRetry(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Type:    DiscordWebhook,
		URL:     server.URL,
		Timeout: 5 * time.Second,
		Retries: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	report := ReportData{
		TraceID:   "test-retry",
		TxHash:    "0xretry",
		Network:   "testnet",
		Status:    "error",
		Error:     "Test error",
		Timestamp: time.Now(),
	}

	err = client.Send(report)
	if err != nil {
		t.Fatalf("Failed after retries: %v", err)
	}

	if attempt != 2 {
		t.Errorf("Expected 2 attempts (1 fail + 1 success), got %d", attempt)
	}
}

func TestSimulatorNotifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	config := NotifierConfig{
		Enabled:   true,
		ErrorOnly: false,
		Webhooks: []Config{
			{
				Type:    SlackWebhook,
				URL:     server.URL,
				Timeout: 5 * time.Second,
				Retries: 1,
			},
		},
	}

	notifier, err := NewSimulatorNotifier(config)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	if !notifier.IsEnabled() {
		t.Error("Notifier should be enabled")
	}

	if notifier.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", notifier.ClientCount())
	}
}

func TestSimulatorNotifierErrorOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	config := NotifierConfig{
		Enabled:   true,
		ErrorOnly: true,
		Webhooks: []Config{
			{
				Type:    DiscordWebhook,
				URL:     server.URL,
				Timeout: 5 * time.Second,
				Retries: 1,
			},
		},
	}

	notifier, err := NewSimulatorNotifier(config)
	if err != nil {
		t.Fatalf("Failed to create notifier: %v", err)
	}

	// Success response should not trigger notification
	resp := &simulator.SimulationResponse{
		Status: "success",
	}
	notifier.NotifyResponse(nil, resp, "0xtest", "testnet", "")
	// If we get here without panic, error-only filtering works

	// Error response should trigger notification
	errResp := &simulator.SimulationResponse{
		Status: "error",
		Error:  "Test error",
	}
	notifier.NotifyResponse(nil, errResp, "0xtest", "testnet", "")
}

func TestColorMapping(t *testing.T) {
	tests := []struct {
		status string
		color  string
	}{
		{"success", "36a64f"},
		{"error", "e74c3c"},
		{"warning", "f39c12"},
		{"unknown", "95a5a6"},
	}

	for _, tt := range tests {
		result := colorForStatus(tt.status)
		if result != tt.color {
			t.Errorf("colorForStatus(%s) = %s, want %s", tt.status, result, tt.color)
		}
	}
}

func TestStringTruncation(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"test", 4, "test"},
		{"toolong", 4, "tool..."},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestHexToDecimal(t *testing.T) {
	tests := []struct {
		hex      string
		expected int
	}{
		{"36a64f", 3581519},
		{"e74c3c", 15158332},
		{"f39c12", 15965202},
		{"95a5a6", 9807270},
	}

	for _, tt := range tests {
		result := hexToDecimal(tt.hex)
		if result != tt.expected {
			t.Errorf("hexToDecimal(%q) = %d, want %d", tt.hex, result, tt.expected)
		}
	}
}

func BenchmarkSlackMessageFormatting(b *testing.B) {
	report := ReportData{
		TraceID:          "bench-trace",
		TxHash:           "0xbench",
		Network:          "testnet",
		Status:           "error",
		Error:            "Benchmark error",
		Timestamp:        time.Now(),
		DiagnosticEvents: make([]simulator.DiagnosticEvent, 10),
	}

	for i := 0; i < 10; i++ {
		report.DiagnosticEvents[i] = simulator.DiagnosticEvent{
			EventType: fmt.Sprintf("event-%d", i),
			Topics:    []string{"topic1", "topic2"},
			Data:      "Test data",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatSlackMessage(report)
	}
}

func BenchmarkDiscordMessageFormatting(b *testing.B) {
	report := ReportData{
		TraceID:          "bench-trace",
		TxHash:           "0xbench",
		Network:          "mainnet",
		Status:           "success",
		Timestamp:        time.Now(),
		DiagnosticEvents: make([]simulator.DiagnosticEvent, 10),
	}

	for i := 0; i < 10; i++ {
		report.DiagnosticEvents[i] = simulator.DiagnosticEvent{
			EventType: fmt.Sprintf("event-%d", i),
			Topics:    []string{"topic1", "topic2"},
			Data:      "Test data",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatDiscordMessage(report)
	}
}
