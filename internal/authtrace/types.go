// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package authtrace

import "encoding/json"

type SignatureType string

const (
	Ed25519       SignatureType = "ed25519"
	Secp256k1     SignatureType = "secp256k1"
	PreAuthorized SignatureType = "pre_authorized"
	CustomAccount SignatureType = "custom_account"
)

type AuthFailureReason string

const (
	ReasonMissingSignature     AuthFailureReason = "missing_signature"
	ReasonInvalidSignature     AuthFailureReason = "invalid_signature"
	ReasonThresholdNotMet      AuthFailureReason = "threshold_not_met"
	ReasonWeightInsufficient   AuthFailureReason = "weight_insufficient"
	ReasonInvalidPublicKey     AuthFailureReason = "invalid_public_key"
	ReasonExpiredPreAuth       AuthFailureReason = "expired_pre_auth"
	ReasonCustomContractFailed AuthFailureReason = "custom_contract_failed"
	ReasonReplayAttackDetected AuthFailureReason = "replay_attack_detected" // #1217
	ReasonUnknown              AuthFailureReason = "unknown"
)

type KeyWeight struct {
	PublicKey string        `json:"public_key"`
	Weight    uint32        `json:"weight"`
	Type      SignatureType `json:"type"`
}

type SignerInfo struct {
	AccountID      string        `json:"account_id"`
	Address        string        `json:"address,omitempty"` // Soroban account/contract address form.
	SignerKey      string        `json:"signer_key"`
	SignerType     SignatureType `json:"signer_type"`
	Weight         uint32        `json:"weight"`
	VerificationID string        `json:"verification_id,omitempty"`
}

type ThresholdConfig struct {
	LowThreshold    uint32 `json:"low_threshold"`
	MediumThreshold uint32 `json:"medium_threshold"`
	HighThreshold   uint32 `json:"high_threshold"`
}

type AuthEvent struct {
	Timestamp     int64             `json:"timestamp"`
	EventType     string            `json:"event_type"`
	AccountID     string            `json:"account_id"`
	Address       string            `json:"address,omitempty"` // Supports account and contract addresses.
	SignerKey     string            `json:"signer_key,omitempty"`
	SignatureType SignatureType     `json:"signature_type,omitempty"`
	Weight        uint32            `json:"weight,omitempty"`
	Status        string            `json:"status"`
	Details       string            `json:"details,omitempty"`
	ErrorReason   AuthFailureReason `json:"error_reason,omitempty"`
}

type AuthFailure struct {
	AccountID       string            `json:"account_id"`
	FailureReason   AuthFailureReason `json:"failure_reason"`
	RequiredWeight  uint32            `json:"required_weight"`
	CollectedWeight uint32            `json:"collected_weight"`
	MissingWeight   uint32            `json:"missing_weight"`
	TotalSigners    uint32            `json:"total_signers"`
	ValidSigners    uint32            `json:"valid_signers"`
	FailedSigners   []SignerInfo      `json:"failed_signers"`
	DetailedTrace   []AuthEvent       `json:"detailed_trace"`
}

type AuthTrace struct {
	Success          bool                 `json:"success"`
	AccountID        string               `json:"account_id"`
	SignerCount      uint32               `json:"signer_count"`
	ValidSignatures  uint32               `json:"valid_signatures"`
	SignatureWeights []KeyWeight          `json:"signature_weights"`
	Thresholds       ThresholdConfig      `json:"thresholds"`
	AuthEvents       []AuthEvent          `json:"auth_events"`
	Failures         []AuthFailure        `json:"failures"`
	CustomContracts  []CustomContractAuth `json:"custom_contracts,omitempty"`
}

// ToJSON serialises an AuthTrace to indented JSON for use by external audit
// tools (#1213). The output is stable and safe for both file storage and
// API responses.
func (a *AuthTrace) ToJSON() ([]byte, error) {
	out, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ToJSONString is a convenience wrapper that returns the JSON as a string.
// Returns an empty string and the error if marshalling fails.
func (a *AuthTrace) ToJSONString() (string, error) {
	b, err := a.ToJSON()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type CustomContractAuth struct {
	ContractID string   `json:"contract_id"`
	Method     string   `json:"method"`
	Params     []string `json:"params,omitempty"`
	Result     string   `json:"result"`
	ErrorMsg   string   `json:"error_msg,omitempty"`
}

// SACCall holds metadata for a Stellar Asset Contract interaction (#1210).
type SACCall struct {
	ContractID string   `json:"contract_id"`
	AssetLabel string   `json:"asset_label"` // e.g. "XLM (Native)" or "USDC (Circle)"
	Method     string   `json:"method"`
	Params     []string `json:"params,omitempty"`
	Result     string   `json:"result"`
	ErrorMsg   string   `json:"error_msg,omitempty"`
}

// ReplayAttackWarning is returned by CheckReplayAttack when a vulnerability is
// detected (#1217). It carries enough context for the caller to log or surface
// the issue without having to re-inspect the event stream.
type ReplayAttackWarning struct {
	AccountID   string `json:"account_id"`
	AntiPattern string `json:"anti_pattern"` // "missing_nonce" | "duplicate_nonce" | "stale_nonce_timestamp"
	Detail      string `json:"detail"`
	DetectedAt  int64  `json:"detected_at_ms"`
}

type Config struct {
	TraceCustomContracts bool
	CaptureSigDetails    bool
	MaxEventDepth        int
}
