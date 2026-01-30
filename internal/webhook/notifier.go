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
	"fmt"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/simulator"
)

// SimulatorNotifier handles notifications for CI session failures
type SimulatorNotifier struct {
	clients   []*Client
	enabled   bool
	errorOnly bool
}

// NotifierConfig contains configuration for the notifier
type NotifierConfig struct {
	Enabled   bool
	ErrorOnly bool
	Webhooks  []Config
}

// NewSimulatorNotifier creates a notifier for simulator session events
func NewSimulatorNotifier(config NotifierConfig) (*SimulatorNotifier, error) {
	if !config.Enabled || len(config.Webhooks) == 0 {
		return &SimulatorNotifier{
			enabled: false,
		}, nil
	}

	clients := make([]*Client, 0, len(config.Webhooks))
	for _, whConfig := range config.Webhooks {
		client, err := NewClient(whConfig)
		if err != nil {
			logger.Logger.Warn(
				"Failed to create webhook client",
				"type", whConfig.Type,
				"error", err,
			)
			continue
		}
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no valid webhook clients could be created")
	}

	return &SimulatorNotifier{
		clients:   clients,
		enabled:   true,
		errorOnly: config.ErrorOnly,
	}, nil
}

// NotifyResponse sends a notification based on the simulation response
func (sn *SimulatorNotifier) NotifyResponse(
	req *simulator.SimulationRequest,
	resp *simulator.SimulationResponse,
	txHash string,
	network string,
	auditLogURL string,
) {
	if !sn.enabled {
		return
	}

	// Skip if error-only mode and status is success
	if sn.errorOnly && resp.Status == "success" {
		return
	}

	report := sn.buildReportData(req, resp, txHash, network, auditLogURL)
	sn.notifyAll(report)
}

// NotifyError sends an error notification directly
func (sn *SimulatorNotifier) NotifyError(
	txHash string,
	network string,
	errorMsg string,
	auditLogURL string,
) {
	if !sn.enabled {
		return
	}

	report := ReportData{
		TraceID:     "error-" + fmt.Sprintf("%d", time.Now().Unix()),
		TxHash:      txHash,
		Network:     network,
		Status:      "error",
		Error:       errorMsg,
		Timestamp:   time.Now(),
		AuditLogURL: auditLogURL,
	}

	sn.notifyAll(report)
}

// buildReportData constructs the ReportData from simulator response
func (sn *SimulatorNotifier) buildReportData(
	req *simulator.SimulationRequest,
	resp *simulator.SimulationResponse,
	txHash string,
	network string,
	auditLogURL string,
) ReportData {
	report := ReportData{
		TraceID:          "trace-" + fmt.Sprintf("%d", time.Now().Unix()),
		TxHash:           txHash,
		Network:          network,
		Status:           resp.Status,
		Error:            resp.Error,
		Timestamp:        time.Now(),
		AuditLogURL:      auditLogURL,
		DiagnosticEvents: resp.DiagnosticEvents,
		Logs:             resp.Logs,
	}

	return report
}

// notifyAll sends the report to all configured webhooks
func (sn *SimulatorNotifier) notifyAll(report ReportData) {
	if len(sn.clients) == 0 {
		return
	}

	for _, client := range sn.clients {
		go func(c *Client) {
			if err := c.Send(report); err != nil {
				logger.Logger.Error(
					"Failed to send webhook notification",
					"type", c.config.Type,
					"error", err,
				)
			}
		}(client)
	}
}

// IsEnabled returns whether notifications are enabled
func (sn *SimulatorNotifier) IsEnabled() bool {
	return sn.enabled
}

// ClientCount returns the number of configured webhook clients
func (sn *SimulatorNotifier) ClientCount() int {
	return len(sn.clients)
}
