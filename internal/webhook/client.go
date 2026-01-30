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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

// WebhookType defines the supported webhook platforms
type WebhookType string

const (
	SlackWebhook   WebhookType = "slack"
	DiscordWebhook WebhookType = "discord"
)

// Config represents webhook configuration
type Config struct {
	Type    WebhookType
	URL     string
	Timeout time.Duration
	Retries int
}

// Client handles webhook delivery
type Client struct {
	config     Config
	httpClient *http.Client
}

// NewClient creates a new webhook client with validation
func NewClient(config Config) (*Client, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("webhook URL cannot be empty")
	}

	if _, err := url.Parse(config.URL); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.Retries < 0 {
		config.Retries = 3
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// Send delivers the debugging report to the webhook
func (c *Client) Send(report ReportData) error {
	var payload interface{}

	switch c.config.Type {
	case SlackWebhook:
		payload = FormatSlackMessage(report)
	case DiscordWebhook:
		payload = FormatDiscordMessage(report)
	default:
		return fmt.Errorf("unsupported webhook type: %s", c.config.Type)
	}

	return c.sendWithRetry(payload)
}

// sendWithRetry attempts to send the webhook with exponential backoff
func (c *Client) sendWithRetry(payload interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.Retries; attempt++ {
		if attempt > 0 {
			backoffDuration := time.Duration(1<<uint(attempt)) * time.Second
			logger.Logger.Debug(
				"Retrying webhook send",
				"attempt", attempt+1,
				"backoff", backoffDuration.String(),
			)
			time.Sleep(backoffDuration)
		}

		err := c.sendRequest(payload)
		if err == nil {
			return nil
		}

		lastErr = err
		logger.Logger.Warn(
			"Webhook send failed",
			"attempt", attempt+1,
			"error", err,
		)
	}

	return fmt.Errorf("webhook delivery failed after %d attempts: %w", c.config.Retries+1, lastErr)
}

// sendRequest performs the actual HTTP POST to the webhook
func (c *Client) sendRequest(payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ERST-Debugger/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"webhook returned status %d: %s",
			resp.StatusCode,
			string(respBody),
		)
	}

	logger.Logger.Debug(
		"Webhook sent successfully",
		"type", c.config.Type,
		"status", resp.StatusCode,
	)

	return nil
}

// Validate checks if the webhook configuration is valid
func (c *Client) Validate() error {
	testReport := ReportData{
		TraceID:   "test-trace-id",
		TxHash:    "test-tx-hash",
		Network:   "testnet",
		Status:    "success",
		Timestamp: time.Now(),
	}

	return c.Send(testReport)
}
