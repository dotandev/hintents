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

package security

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"github.com/stellar/go/xdr"
)

// Severity levels for security findings
type Severity string

const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
	SeverityInfo   Severity = "INFO"
)

// FindingType categorizes the security issue
type FindingType string

const (
	FindingVerifiedRisk  FindingType = "VERIFIED_RISK"
	FindingHeuristicWarn FindingType = "HEURISTIC_WARNING"
)

// Finding represents a security vulnerability or warning
type Finding struct {
	Type        FindingType `json:"type"`
	Severity    Severity    `json:"severity"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Evidence    string      `json:"evidence,omitempty"`
}

// Detector analyzes transactions for security vulnerabilities
type Detector struct {
	findings []Finding
}

// NewDetector creates a new security detector
func NewDetector() *Detector {
	return &Detector{
		findings: make([]Finding, 0),
	}
}

// Analyze performs security checks on transaction data
func (d *Detector) Analyze(envelopeXdr, resultMetaXdr string, events []string, logs []string) []Finding {
	d.findings = make([]Finding, 0)

	// Decode envelope
	envelope, err := decodeEnvelope(envelopeXdr)
	if err == nil {
		d.checkLargeValueTransfers(envelope)
		d.checkReentrancyPatterns(envelope, events)
	}

	// Check patterns that don't require envelope
	d.checkIntegerOverflow(events, logs)
	d.checkSuspiciousEvents(events)
	d.checkAuthorizationBypass(events, logs)

	return d.findings
}

// GetFindings returns all detected findings
func (d *Detector) GetFindings() []Finding {
	return d.findings
}

func (d *Detector) addFinding(finding Finding) {
	d.findings = append(d.findings, finding)
}

// checkLargeValueTransfers detects unusually large value transfers
func (d *Detector) checkLargeValueTransfers(envelope xdr.TransactionEnvelope) {
	const largeTransferThreshold = 1000000 * 10000000 // 1M XLM in stroops

	ops := extractOperations(envelope)
	for _, op := range ops {
		switch op.Body.Type {
		case xdr.OperationTypePayment:
			payment := op.Body.PaymentOp
			if payment.Amount > xdr.Int64(largeTransferThreshold) {
				d.addFinding(Finding{
					Type:        FindingHeuristicWarn,
					Severity:    SeverityHigh,
					Title:       "Large Value Transfer Detected",
					Description: fmt.Sprintf("Transfer of %d stroops (%.2f XLM) detected. Verify recipient address.", payment.Amount, float64(payment.Amount)/10000000.0),
					Evidence:    fmt.Sprintf("Destination: %s", payment.Destination.Address()),
				})
			}
		case xdr.OperationTypeInvokeHostFunction:
			// Check for large amounts in contract invocations
			d.checkContractValueTransfer(op)
		}
	}
}

// checkContractValueTransfer analyzes contract invocations for large transfers
func (d *Detector) checkContractValueTransfer(op xdr.Operation) {
	hostFn := op.Body.InvokeHostFunctionOp
	if hostFn == nil {
		return
	}

	if hostFn.HostFunction.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
		invokeArgs := hostFn.HostFunction.InvokeContract
		if invokeArgs == nil {
			return
		}

		// Look for amount parameters (common in transfer functions)
		for _, arg := range invokeArgs.Args {
			if arg.Type == xdr.ScValTypeScvI128 || arg.Type == xdr.ScValTypeScvU128 {
				amount := extractAmount(arg)
				if amount != nil && amount.Cmp(big.NewInt(100000000000000)) > 0 { // 10M tokens (assuming 7 decimals)
					d.addFinding(Finding{
						Type:        FindingHeuristicWarn,
						Severity:    SeverityMedium,
						Title:       "Large Contract Value Transfer",
						Description: fmt.Sprintf("Contract invocation with large amount: %s", amount.String()),
						Evidence:    "Review contract address and function parameters",
					})
				}
			}
		}
	}
}

// checkReentrancyPatterns detects potential reentrancy vulnerabilities
func (d *Detector) checkReentrancyPatterns(envelope xdr.TransactionEnvelope, events []string) {
	ops := extractOperations(envelope)

	// Count contract invocations
	invocationCount := 0
	for _, op := range ops {
		if op.Body.Type == xdr.OperationTypeInvokeHostFunction {
			invocationCount++
		}
	}

	// Multiple invocations + state changes = potential reentrancy
	if invocationCount > 1 {
		hasStateChange := false
		for _, event := range events {
			if strings.Contains(event, "contract_data") || strings.Contains(event, "write") {
				hasStateChange = true
				break
			}
		}

		if hasStateChange {
			d.addFinding(Finding{
				Type:        FindingHeuristicWarn,
				Severity:    SeverityMedium,
				Title:       "Potential Reentrancy Pattern",
				Description: fmt.Sprintf("Transaction contains %d contract invocations with state changes. Verify reentrancy guards are in place.", invocationCount),
				Evidence:    "Multiple contract calls with storage modifications detected",
			})
		}
	}
}

// checkIntegerOverflow detects potential integer overflow issues
func (d *Detector) checkIntegerOverflow(events []string, logs []string) {
	overflowKeywords := []string{"overflow", "underflow"}
	arithmeticKeywords := []string{"checked_add", "checked_sub", "checked_mul", "checked_div", "arithmetic"}

	for _, log := range logs {
		logLower := strings.ToLower(log)

		// Check for explicit overflow/underflow mentions
		for _, keyword := range overflowKeywords {
			if strings.Contains(logLower, keyword) {
				d.addFinding(Finding{
					Type:        FindingVerifiedRisk,
					Severity:    SeverityHigh,
					Title:       "Integer Overflow/Underflow Detected",
					Description: "Arithmetic operation failed, indicating potential overflow or underflow",
					Evidence:    log,
				})
				return
			}
		}

		// Check for arithmetic operation failures
		for _, keyword := range arithmeticKeywords {
			if strings.Contains(logLower, keyword) && (strings.Contains(logLower, "fail") || strings.Contains(logLower, "error")) {
				d.addFinding(Finding{
					Type:        FindingVerifiedRisk,
					Severity:    SeverityHigh,
					Title:       "Integer Overflow/Underflow Detected",
					Description: "Arithmetic operation failed, indicating potential overflow or underflow",
					Evidence:    log,
				})
				return
			}
		}
	}
}

// checkSuspiciousEvents analyzes diagnostic events for suspicious patterns
func (d *Detector) checkSuspiciousEvents(events []string) {
	for _, event := range events {
		eventLower := strings.ToLower(event)

		// Check for authorization failures
		if strings.Contains(eventLower, "auth") && (strings.Contains(eventLower, "fail") || strings.Contains(eventLower, "invalid")) {
			d.addFinding(Finding{
				Type:        FindingVerifiedRisk,
				Severity:    SeverityHigh,
				Title:       "Authorization Failure",
				Description: "Contract authorization check failed",
				Evidence:    event,
			})
		}

		// Check for panic/trap events
		if strings.Contains(eventLower, "panic") || strings.Contains(eventLower, "trap") {
			d.addFinding(Finding{
				Type:        FindingVerifiedRisk,
				Severity:    SeverityHigh,
				Title:       "Contract Panic/Trap",
				Description: "Contract execution panicked or trapped",
				Evidence:    event,
			})
		}
	}
}

// checkAuthorizationBypass detects potential authorization bypass attempts
func (d *Detector) checkAuthorizationBypass(events []string, logs []string) {
	hasAuthCheck := false
	hasPrivilegedOp := false

	for _, log := range logs {
		logLower := strings.ToLower(log)
		if strings.Contains(logLower, "require_auth") || strings.Contains(logLower, "check_auth") {
			hasAuthCheck = true
		}
		if strings.Contains(logLower, "admin") || strings.Contains(logLower, "owner") || strings.Contains(logLower, "privileged") {
			hasPrivilegedOp = true
		}
	}

	// Privileged operation without auth check
	if hasPrivilegedOp && !hasAuthCheck {
		d.addFinding(Finding{
			Type:        FindingHeuristicWarn,
			Severity:    SeverityHigh,
			Title:       "Potential Authorization Bypass",
			Description: "Privileged operation detected without corresponding authorization check",
			Evidence:    "Review contract authorization logic",
		})
	}
}

// Helper functions

func decodeEnvelope(envelopeXdr string) (xdr.TransactionEnvelope, error) {
	var envelope xdr.TransactionEnvelope
	decoded, err := base64.StdEncoding.DecodeString(envelopeXdr)
	if err != nil {
		return envelope, err
	}
	_, err = xdr.Unmarshal(strings.NewReader(string(decoded)), &envelope)
	return envelope, err
}

func extractOperations(envelope xdr.TransactionEnvelope) []xdr.Operation {
	switch envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		return envelope.V1.Tx.Operations
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		return envelope.V0.Tx.Operations
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		return envelope.FeeBump.Tx.InnerTx.V1.Tx.Operations
	}
	return nil
}

func extractAmount(val xdr.ScVal) *big.Int {
	switch val.Type {
	case xdr.ScValTypeScvI128:
		parts := val.I128
		if parts == nil {
			return nil
		}
		amount := new(big.Int).SetInt64(int64(parts.Hi))
		amount.Lsh(amount, 64)
		amount.Add(amount, new(big.Int).SetUint64(uint64(parts.Lo)))
		return amount
	case xdr.ScValTypeScvU128:
		parts := val.U128
		if parts == nil {
			return nil
		}
		amount := new(big.Int).SetUint64(uint64(parts.Hi))
		amount.Lsh(amount, 64)
		amount.Add(amount, new(big.Int).SetUint64(uint64(parts.Lo)))
		return amount
	}
	return nil
}
