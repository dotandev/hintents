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
)

type ContractAuthHandler interface {
	ValidateAuth(contractID string, method string, params []interface{}) (bool, error)
	GetAuthName() string
	GetAuthDetails() map[string]interface{}
}

type CustomContractAuthValidator struct {
	contracts map[string]ContractAuthHandler
}

func NewCustomContractAuthValidator() *CustomContractAuthValidator {
	return &CustomContractAuthValidator{
		contracts: make(map[string]ContractAuthHandler),
	}
}

func (v *CustomContractAuthValidator) RegisterContract(contractID string, handler ContractAuthHandler) error {
	if contractID == "" {
		return fmt.Errorf("contract ID cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	v.contracts[contractID] = handler
	return nil
}

func (v *CustomContractAuthValidator) UnregisterContract(contractID string) {
	delete(v.contracts, contractID)
}

func (v *CustomContractAuthValidator) ValidateContract(contractID string, method string, params []interface{}) (bool, error) {
	handler, ok := v.contracts[contractID]
	if !ok {
		return false, fmt.Errorf("no handler registered for contract: %s", contractID)
	}
	return handler.ValidateAuth(contractID, method, params)
}

func (v *CustomContractAuthValidator) GetContractInfo(contractID string) (map[string]interface{}, error) {
	handler, ok := v.contracts[contractID]
	if !ok {
		return nil, fmt.Errorf("contract not found: %s", contractID)
	}
	return map[string]interface{}{
		"contract_id": contractID,
		"auth_name":   handler.GetAuthName(),
		"details":     handler.GetAuthDetails(),
	}, nil
}

func (v *CustomContractAuthValidator) ListContracts() []string {
	contracts := make([]string, 0, len(v.contracts))
	for id := range v.contracts {
		contracts = append(contracts, id)
	}
	return contracts
}

type MultiSigContractAuth struct {
	RequiredSignatures int
	SignerThreshold    uint32
	Signers            map[string]uint32
}

func NewMultiSigContractAuth(required int, threshold uint32, signers map[string]uint32) *MultiSigContractAuth {
	if signers == nil {
		signers = make(map[string]uint32)
	}
	return &MultiSigContractAuth{
		RequiredSignatures: required,
		SignerThreshold:    threshold,
		Signers:            signers,
	}
}

func (m *MultiSigContractAuth) GetAuthName() string {
	return "multi_sig"
}

func (m *MultiSigContractAuth) GetAuthDetails() map[string]interface{} {
	return map[string]interface{}{
		"required_signatures": m.RequiredSignatures,
		"signer_threshold":    m.SignerThreshold,
		"total_signers":       len(m.Signers),
	}
}

func (m *MultiSigContractAuth) ValidateAuth(contractID string, method string, params []interface{}) (bool, error) {
	if len(params) < 1 {
		return false, fmt.Errorf("insufficient parameters for multi-sig validation")
	}

	signaturesData, ok := params[0].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("invalid signatures data format")
	}

	signatures, ok := signaturesData["signatures"].([]interface{})
	if !ok {
		return false, fmt.Errorf("signatures field not found or invalid type")
	}

	if len(signatures) < m.RequiredSignatures {
		return false, fmt.Errorf("insufficient signatures: got %d, required %d", len(signatures), m.RequiredSignatures)
	}

	totalWeight := uint32(0)
	for _, sig := range signatures {
		sigData, ok := sig.(map[string]interface{})
		if !ok {
			continue
		}

		signerKey, ok := sigData["signer_key"].(string)
		if !ok {
			continue
		}

		weight, ok := m.Signers[signerKey]
		if !ok {
			continue
		}

		totalWeight += weight
	}

	return totalWeight >= m.SignerThreshold, nil
}

type RecoveryAuth struct {
	RecoveryKey string
	Delay       uint64
}

func NewRecoveryAuth(recoveryKey string, delay uint64) *RecoveryAuth {
	return &RecoveryAuth{
		RecoveryKey: recoveryKey,
		Delay:       delay,
	}
}

func (r *RecoveryAuth) GetAuthName() string {
	return "recovery"
}

func (r *RecoveryAuth) GetAuthDetails() map[string]interface{} {
	return map[string]interface{}{
		"recovery_key": r.RecoveryKey,
		"delay_ms":     r.Delay,
	}
}

func (r *RecoveryAuth) ValidateAuth(contractID string, method string, params []interface{}) (bool, error) {
	if len(params) < 2 {
		return false, fmt.Errorf("insufficient parameters for recovery validation")
	}

	recoveryKeyParam, ok := params[0].(string)
	if !ok {
		return false, fmt.Errorf("invalid recovery key format")
	}

	if recoveryKeyParam != r.RecoveryKey {
		return false, fmt.Errorf("recovery key mismatch")
	}

	return true, nil
}

func UnmarshalCustomContractAuth(data []byte) (*CustomContractAuthValidator, error) {
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal custom contract config: %w", err)
	}

	validator := NewCustomContractAuthValidator()

	for contractID, authConfig := range config {
		authData, err := json.Marshal(authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal contract config: %w", err)
		}

		var auth map[string]interface{}
		if err := json.Unmarshal(authData, &auth); err != nil {
			return nil, fmt.Errorf("invalid auth config for contract %s: %w", contractID, err)
		}

		authType, ok := auth["type"].(string)
		if !ok {
			return nil, fmt.Errorf("missing auth type for contract %s", contractID)
		}

		var handler ContractAuthHandler

		switch authType {
		case "multi_sig":
			required := int(auth["required_signatures"].(float64))
			threshold := uint32(auth["signer_threshold"].(float64))
			signers := make(map[string]uint32)
			handler = NewMultiSigContractAuth(required, threshold, signers)

		case "recovery":
			recoveryKey := auth["recovery_key"].(string)
			delay := uint64(auth["delay"].(float64))
			handler = NewRecoveryAuth(recoveryKey, delay)

		default:
			return nil, fmt.Errorf("unknown auth type for contract %s: %s", contractID, authType)
		}

		if err := validator.RegisterContract(contractID, handler); err != nil {
			return nil, err
		}
	}

	return validator, nil
}
