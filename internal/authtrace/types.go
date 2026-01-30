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
	ReasonUnknown              AuthFailureReason = "unknown"
)

type KeyWeight struct {
	PublicKey string        `json:"public_key"`
	Weight    uint32        `json:"weight"`
	Type      SignatureType `json:"type"`
}

type SignerInfo struct {
	AccountID      string        `json:"account_id"`
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

type CustomContractAuth struct {
	ContractID string   `json:"contract_id"`
	Method     string   `json:"method"`
	Params     []string `json:"params,omitempty"`
	Result     string   `json:"result"`
	ErrorMsg   string   `json:"error_msg,omitempty"`
}

type AuthTraceConfig struct {
	TraceCustomContracts bool
	CaptureSigDetails    bool
	MaxEventDepth        int
}
