// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
)

// ValidationError represents a schema validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e *ValidationError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] validation error on field '%s': %s", e.Code, e.Field, e.Message)
	}
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// Validator validates simulation requests against formal schemas
type Validator struct {
	strictMode       bool
	enabledRules     map[string]bool
	customValidators map[string]func(interface{}) error
}

// NewValidator creates a new validator
func NewValidator(strictMode bool) *Validator {
	return &Validator{
		strictMode:       strictMode,
		enabledRules:     make(map[string]bool),
		customValidators: make(map[string]func(interface{}) error),
	}
}

// WithRule enables a specific validation rule
func (v *Validator) WithRule(ruleName string) *Validator {
	v.enabledRules[ruleName] = true
	return v
}

// WithCustomValidator adds a custom validator for a field
func (v *Validator) WithCustomValidator(field string, validator func(interface{}) error) *Validator {
	v.customValidators[field] = validator
	return v
}

// ValidateRequest validates a simulation request against formal schema
func (v *Validator) ValidateRequest(req *SimulationRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil", Code: "ERR_NULL_REQUEST"}
	}

	// Validate against JSON schema structure
	if err := v.validateSchema(req); err != nil {
		return err
	}

	// Validate required fields
	if err := v.validateEnvelopeXdr(req.EnvelopeXdr); err != nil {
		return err
	}

	if err := v.validateResultMetaXdr(req.ResultMetaXdr); err != nil {
		return err
	}

	// Validate optional fields
	if req.LedgerEntries != nil {
		if err := v.validateLedgerEntries(req.LedgerEntries); err != nil {
			return err
		}
	}

	if req.ProtocolVersion != nil {
		if err := v.validateProtocolVersion(*req.ProtocolVersion); err != nil {
			return err
		}
	}

	if req.ResourceCalibration != nil {
		if err := v.validateResourceCalibration(req.ResourceCalibration); err != nil {
			return err
		}
	}

	if req.AuthTraceOpts != nil {
		if err := v.validateAuthTraceOptions(req.AuthTraceOpts); err != nil {
			return err
		}
	}

	if req.Timestamp != 0 {
		if err := v.validateTimestamp(req.Timestamp); err != nil {
			return err
		}
	}

	if req.LedgerSequence != 0 {
		if err := v.validateLedgerSequence(req.LedgerSequence); err != nil {
			return err
		}
	}

	// Run custom validators
	for field, validator := range v.customValidators {
		if err := validator(req); err != nil {
			return &ValidationError{Field: field, Message: err.Error(), Code: "ERR_CUSTOM_VALIDATION"}
		}
	}

	return nil
}

// validateSchema validates the request structure against JSON schema
func (v *Validator) validateSchema(req *SimulationRequest) error {
	// Marshal and unmarshal to ensure JSON compatibility
	data, err := json.Marshal(req)
	if err != nil {
		return &ValidationError{Field: "request", Message: "failed to serialize", Code: "ERR_SERIALIZATION"}
	}

	var test SimulationRequest
	if err := json.Unmarshal(data, &test); err != nil {
		return &ValidationError{Field: "request", Message: "failed to deserialize", Code: "ERR_DESERIALIZATION"}
	}

	return nil
}

// validateEnvelopeXdr validates the envelope XDR field
func (v *Validator) validateEnvelopeXdr(xdr string) error {
	if xdr == "" {
		return &ValidationError{Field: "envelope_xdr", Message: "cannot be empty", Code: "ERR_EMPTY_FIELD"}
	}

	if !isValidBase64(xdr) {
		return &ValidationError{Field: "envelope_xdr", Message: "must be valid base64", Code: "ERR_INVALID_BASE64"}
	}

	if v.strictMode && len(xdr) > 1024*1024 {
		return &ValidationError{Field: "envelope_xdr", Message: "exceeds maximum size of 1MB", Code: "ERR_SIZE_LIMIT"}
	}

	// Validate XDR structure if rule is enabled
	if v.enabledRules["validate_xdr_structure"] {
		if err := v.validateXDRStructure(xdr); err != nil {
			return err
		}
	}

	return nil
}

// validateResultMetaXdr validates the result meta XDR field
func (v *Validator) validateResultMetaXdr(xdr string) error {
	if xdr == "" {
		return &ValidationError{Field: "result_meta_xdr", Message: "cannot be empty", Code: "ERR_EMPTY_FIELD"}
	}

	if !isValidBase64(xdr) {
		return &ValidationError{Field: "result_meta_xdr", Message: "must be valid base64", Code: "ERR_INVALID_BASE64"}
	}

	if v.strictMode && len(xdr) > 10*1024*1024 {
		return &ValidationError{Field: "result_meta_xdr", Message: "exceeds maximum size of 10MB", Code: "ERR_SIZE_LIMIT"}
	}

	return nil
}

// validateLedgerEntries validates ledger entries map
func (v *Validator) validateLedgerEntries(entries map[string]string) error {
	if len(entries) == 0 {
		return nil
	}

	if v.strictMode && len(entries) > 10000 {
		return &ValidationError{Field: "ledger_entries", Message: "exceeds maximum of 10000 entries", Code: "ERR_SIZE_LIMIT"}
	}

	for key, value := range entries {
		if key == "" {
			return &ValidationError{Field: "ledger_entries", Message: "key cannot be empty", Code: "ERR_EMPTY_KEY"}
		}

		if !isValidBase64(key) {
			return &ValidationError{Field: "ledger_entries", Message: fmt.Sprintf("key '%s' must be valid base64", truncate(key, 20)), Code: "ERR_INVALID_BASE64"}
		}

		if value == "" {
			return &ValidationError{Field: "ledger_entries", Message: fmt.Sprintf("value for key '%s' cannot be empty", truncate(key, 20)), Code: "ERR_EMPTY_VALUE"}
		}

		if !isValidBase64(value) {
			return &ValidationError{Field: "ledger_entries", Message: fmt.Sprintf("value for key '%s' must be valid base64", truncate(key, 20)), Code: "ERR_INVALID_BASE64"}
		}
	}

	return nil
}

// validateProtocolVersion validates protocol version
func (v *Validator) validateProtocolVersion(version uint32) error {
	if version == 0 {
		return &ValidationError{Field: "protocol_version", Message: "cannot be zero", Code: "ERR_INVALID_VERSION"}
	}

	if v.strictMode && version > 100 {
		return &ValidationError{Field: "protocol_version", Message: "exceeds maximum supported version", Code: "ERR_VERSION_TOO_HIGH"}
	}

	return nil
}

// validateResourceCalibration validates resource calibration
func (v *Validator) validateResourceCalibration(cal *ResourceCalibration) error {
	if cal == nil {
		return nil
	}

	if cal.SHA256Fixed == 0 && cal.SHA256PerByte == 0 &&
		cal.Keccak256Fixed == 0 && cal.Keccak256PerByte == 0 &&
		cal.Ed25519Fixed == 0 {
		return &ValidationError{Field: "resource_calibration", Message: "all fields cannot be zero", Code: "ERR_INVALID_CALIBRATION"}
	}

	// Validate reasonable ranges
	if v.strictMode {
		if cal.SHA256Fixed > 1000000 {
			return &ValidationError{Field: "resource_calibration.sha256_fixed", Message: "exceeds reasonable limit", Code: "ERR_VALUE_TOO_HIGH"}
		}
		if cal.SHA256PerByte > 10000 {
			return &ValidationError{Field: "resource_calibration.sha256_per_byte", Message: "exceeds reasonable limit", Code: "ERR_VALUE_TOO_HIGH"}
		}
	}

	return nil
}

// validateAuthTraceOptions validates auth trace options
func (v *Validator) validateAuthTraceOptions(opts *AuthTraceOptions) error {
	if opts == nil {
		return nil
	}

	if opts.MaxEventDepth < 0 {
		return &ValidationError{Field: "auth_trace_opts.max_event_depth", Message: "cannot be negative", Code: "ERR_NEGATIVE_VALUE"}
	}

	if v.strictMode && opts.MaxEventDepth > 1000 {
		return &ValidationError{Field: "auth_trace_opts.max_event_depth", Message: "exceeds maximum of 1000", Code: "ERR_VALUE_TOO_HIGH"}
	}

	return nil
}

// validateTimestamp validates timestamp field
func (v *Validator) validateTimestamp(timestamp int64) error {
	if timestamp < 0 {
		return &ValidationError{Field: "timestamp", Message: "cannot be negative", Code: "ERR_NEGATIVE_VALUE"}
	}

	if v.strictMode {
		// Check if timestamp is reasonable (not too far in past or future)
		const maxFutureYears = 10
		const maxPastYears = 50
		now := int64(1735689600) // 2025-01-01 as reference
		maxFuture := now + (maxFutureYears * 365 * 24 * 3600)
		maxPast := now - (maxPastYears * 365 * 24 * 3600)

		if timestamp > maxFuture {
			return &ValidationError{Field: "timestamp", Message: "timestamp too far in future", Code: "ERR_INVALID_TIMESTAMP"}
		}
		if timestamp < maxPast {
			return &ValidationError{Field: "timestamp", Message: "timestamp too far in past", Code: "ERR_INVALID_TIMESTAMP"}
		}
	}

	return nil
}

// validateLedgerSequence validates ledger sequence
func (v *Validator) validateLedgerSequence(sequence uint32) error {
	if sequence == 0 {
		return &ValidationError{Field: "ledger_sequence", Message: "cannot be zero", Code: "ERR_INVALID_SEQUENCE"}
	}

	if v.strictMode && sequence > 1000000000 {
		return &ValidationError{Field: "ledger_sequence", Message: "exceeds reasonable limit", Code: "ERR_VALUE_TOO_HIGH"}
	}

	return nil
}

// validateXDRStructure validates XDR structure (basic check)
func (v *Validator) validateXDRStructure(xdr string) error {
	decoded, err := base64.StdEncoding.DecodeString(xdr)
	if err != nil {
		return &ValidationError{Field: "xdr", Message: "invalid base64 encoding", Code: "ERR_INVALID_BASE64"}
	}

	if len(decoded) < 4 {
		return &ValidationError{Field: "xdr", Message: "XDR too short to be valid", Code: "ERR_INVALID_XDR"}
	}

	return nil
}

// ValidateResponse validates a simulation response
func (v *Validator) ValidateResponse(resp *SimulationResponse) error {
	if resp == nil {
		return &ValidationError{Field: "response", Message: "response cannot be nil", Code: "ERR_NULL_RESPONSE"}
	}

	if resp.Status == "" {
		return &ValidationError{Field: "status", Message: "cannot be empty", Code: "ERR_EMPTY_FIELD"}
	}

	if resp.Status != "success" && resp.Status != "error" {
		return &ValidationError{Field: "status", Message: "must be 'success' or 'error'", Code: "ERR_INVALID_STATUS"}
	}

	if resp.Status == "error" && resp.Error == "" {
		return &ValidationError{Field: "error", Message: "must be provided when status is 'error'", Code: "ERR_MISSING_ERROR"}
	}

	if resp.BudgetUsage != nil {
		if err := v.validateBudgetUsage(resp.BudgetUsage); err != nil {
			return err
		}
	}

	if resp.DiagnosticEvents != nil {
		if err := v.validateDiagnosticEvents(resp.DiagnosticEvents); err != nil {
			return err
		}
	}

	return nil
}

// validateBudgetUsage validates budget usage metrics
func (v *Validator) validateBudgetUsage(usage *BudgetUsage) error {
	if usage.CPULimit > 0 && usage.CPUInstructions > usage.CPULimit {
		if !v.strictMode {
			// Warning only in non-strict mode
			return nil
		}
		return &ValidationError{Field: "budget_usage.cpu_instructions", Message: "exceeds CPU limit", Code: "ERR_BUDGET_EXCEEDED"}
	}

	if usage.MemoryLimit > 0 && usage.MemoryBytes > usage.MemoryLimit {
		if !v.strictMode {
			return nil
		}
		return &ValidationError{Field: "budget_usage.memory_bytes", Message: "exceeds memory limit", Code: "ERR_BUDGET_EXCEEDED"}
	}

	if usage.CPUUsagePercent < 0 || usage.CPUUsagePercent > 100 {
		return &ValidationError{Field: "budget_usage.cpu_usage_percent", Message: "must be between 0 and 100", Code: "ERR_INVALID_PERCENTAGE"}
	}

	if usage.MemoryUsagePercent < 0 || usage.MemoryUsagePercent > 100 {
		return &ValidationError{Field: "budget_usage.memory_usage_percent", Message: "must be between 0 and 100", Code: "ERR_INVALID_PERCENTAGE"}
	}

	return nil
}

// validateDiagnosticEvents validates diagnostic events
func (v *Validator) validateDiagnosticEvents(events []DiagnosticEvent) error {
	for i, event := range events {
		if event.EventType == "" {
			return &ValidationError{Field: fmt.Sprintf("diagnostic_events[%d].event_type", i), Message: "cannot be empty", Code: "ERR_EMPTY_FIELD"}
		}

		if event.EventType != "contract" && event.EventType != "system" && event.EventType != "diagnostic" {
			return &ValidationError{Field: fmt.Sprintf("diagnostic_events[%d].event_type", i), Message: "must be 'contract', 'system', or 'diagnostic'", Code: "ERR_INVALID_TYPE"}
		}

		if event.ContractID != nil {
			if err := ValidateContractID(*event.ContractID); err != nil {
				return &ValidationError{Field: fmt.Sprintf("diagnostic_events[%d].contract_id", i), Message: err.Error(), Code: "ERR_INVALID_CONTRACT_ID"}
			}
		}
	}

	return nil
}

// isValidBase64 checks if a string is valid base64
func isValidBase64(s string) bool {
	if s == "" {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// isValidContractID checks if a string is a valid Stellar contract ID
func isValidContractID(id string) bool {
	if len(id) != 56 {
		return false
	}
	if id[0] != 'C' {
		return false
	}
	matched, _ := regexp.MatchString(`^C[A-Z2-7]{55}$`, id)
	return matched
}

// ValidateContractID validates a contract ID
func ValidateContractID(id string) error {
	if id == "" {
		return &ValidationError{Field: "contract_id", Message: "cannot be empty", Code: "ERR_EMPTY_FIELD"}
	}

	if !isValidContractID(id) {
		return &ValidationError{Field: "contract_id", Message: "invalid format (must be 56-char strkey starting with 'C')", Code: "ERR_INVALID_FORMAT"}
	}

	return nil
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
