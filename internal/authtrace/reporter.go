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

package authtrace

import (
	"encoding/json"
	"fmt"
	"strings"
)

type DetailedReporter struct {
	trace *AuthTrace
}

func NewDetailedReporter(trace *AuthTrace) *DetailedReporter {
	return &DetailedReporter{trace: trace}
}

func (r *DetailedReporter) GenerateReport() string {
	var sb strings.Builder

	status := "SUCCEEDED"
	if !r.trace.Success {
		status = "FAILED"
	}

	sb.WriteString(fmt.Sprintf("=== MULTI-SIGNATURE AUTHORIZATION DEBUG REPORT ===\n\n"))
	sb.WriteString(fmt.Sprintf("Authorization: %s\n", status))
	sb.WriteString(fmt.Sprintf("Account: %s\n", r.trace.AccountID))
	sb.WriteString(fmt.Sprintf("Total Signers: %d\n", r.trace.SignerCount))
	sb.WriteString(fmt.Sprintf("Valid Signatures: %d\n\n", r.trace.ValidSignatures))

	if len(r.trace.Failures) > 0 {
		r.writeFailures(&sb)
	}

	if len(r.trace.AuthEvents) > 0 {
		r.writeEvents(&sb)
	}

	if len(r.trace.CustomContracts) > 0 {
		r.writeContracts(&sb)
	}

	return sb.String()
}

func (r *DetailedReporter) writeFailures(sb *strings.Builder) {
	sb.WriteString("--- FAILURE DETAILS ---\n")
	for i, failure := range r.trace.Failures {
		sb.WriteString(fmt.Sprintf("\nFailure #%d:\n", i+1))
		sb.WriteString(fmt.Sprintf("  Reason: %s\n", failure.FailureReason))
		sb.WriteString(fmt.Sprintf("  Required Weight: %d\n", failure.RequiredWeight))
		sb.WriteString(fmt.Sprintf("  Collected Weight: %d\n", failure.CollectedWeight))
		sb.WriteString(fmt.Sprintf("  Missing Weight: %d\n", failure.MissingWeight))

		if len(failure.FailedSigners) > 0 {
			sb.WriteString("  Failed Signers:\n")
			for _, signer := range failure.FailedSigners {
				sb.WriteString(fmt.Sprintf("    - %s (weight: %d, type: %s)\n",
					signer.SignerKey, signer.Weight, signer.SignerType))
			}
		}
	}
}

func (r *DetailedReporter) writeEvents(sb *strings.Builder) {
	sb.WriteString("\n--- AUTHORIZATION TRACE ---\n")
	for i, event := range r.trace.AuthEvents {
		sb.WriteString(fmt.Sprintf("\n[%d] %s\n", i+1, event.EventType))
		if event.SignerKey != "" {
			sb.WriteString(fmt.Sprintf("    Signer: %s\n", event.SignerKey))
		}
		sb.WriteString(fmt.Sprintf("    Status: %s\n", event.Status))
		if event.Weight > 0 {
			sb.WriteString(fmt.Sprintf("    Weight: %d\n", event.Weight))
		}
		if event.Details != "" {
			sb.WriteString(fmt.Sprintf("    Details: %s\n", event.Details))
		}
		if event.ErrorReason != "" {
			sb.WriteString(fmt.Sprintf("    Error: %s\n", event.ErrorReason))
		}
	}
}

func (r *DetailedReporter) writeContracts(sb *strings.Builder) {
	sb.WriteString("\n--- CUSTOM CONTRACT AUTHORIZATIONS ---\n")
	for _, contract := range r.trace.CustomContracts {
		sb.WriteString(fmt.Sprintf("\nContract: %s\n", contract.ContractID))
		sb.WriteString(fmt.Sprintf("  Method: %s\n", contract.Method))
		sb.WriteString(fmt.Sprintf("  Result: %s\n", contract.Result))
		if contract.ErrorMsg != "" {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", contract.ErrorMsg))
		}
	}
}

func (r *DetailedReporter) GenerateJSON() ([]byte, error) {
	return json.MarshalIndent(r.trace, "", "  ")
}

func (r *DetailedReporter) GenerateJSONString() (string, error) {
	data, err := r.GenerateJSON()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *DetailedReporter) SummaryMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"success":          r.trace.Success,
		"account_id":       r.trace.AccountID,
		"total_signers":    r.trace.SignerCount,
		"valid_signatures": r.trace.ValidSignatures,
		"failure_count":    len(r.trace.Failures),
		"event_count":      len(r.trace.AuthEvents),
		"custom_contracts": len(r.trace.CustomContracts),
	}

	if len(r.trace.Failures) > 0 {
		failure := r.trace.Failures[0]
		metrics["failure_reason"] = failure.FailureReason
		metrics["required_weight"] = failure.RequiredWeight
		metrics["collected_weight"] = failure.CollectedWeight
		metrics["missing_weight"] = failure.MissingWeight
	}

	return metrics
}

func (r *DetailedReporter) IdentifyMissingKeys() []SignerInfo {
	if len(r.trace.Failures) == 0 {
		return nil
	}

	failure := r.trace.Failures[0]
	return failure.FailedSigners
}

func (r *DetailedReporter) FindSignatureByKey(key string) *AuthEvent {
	for _, event := range r.trace.AuthEvents {
		if event.SignerKey == key && event.EventType == "signature_verification" {
			return &event
		}
	}
	return nil
}

func (r *DetailedReporter) GetAuthPath(accountID string) []string {
	var path []string
	for _, event := range r.trace.AuthEvents {
		if event.AccountID == accountID {
			path = append(path, fmt.Sprintf("%s(%s)", event.EventType, event.Status))
		}
	}
	return path
}
