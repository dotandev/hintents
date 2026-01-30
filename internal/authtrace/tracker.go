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
	"fmt"
	"sync"
	"time"
)

type Tracker struct {
	mu              sync.RWMutex
	events          []AuthEvent
	failures        []AuthFailure
	config          AuthTraceConfig
	accountContexts map[string]*AccountAuthContext
}

type AccountAuthContext struct {
	AccountID       string
	Signers         map[string]SignerInfo
	ThresholdConfig ThresholdConfig
	CollectedWeight uint32
	WeightByType    map[SignatureType]uint32
}

func NewTracker(config AuthTraceConfig) *Tracker {
	return &Tracker{
		events:          make([]AuthEvent, 0),
		failures:        make([]AuthFailure, 0),
		config:          config,
		accountContexts: make(map[string]*AccountAuthContext),
	}
}

func (t *Tracker) RecordEvent(event AuthEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}

	if t.config.MaxEventDepth > 0 && len(t.events) >= t.config.MaxEventDepth {
		return
	}

	t.events = append(t.events, event)
}

func (t *Tracker) InitializeAccountContext(accountID string, signers []SignerInfo, thresholds ThresholdConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ctx := &AccountAuthContext{
		AccountID:       accountID,
		Signers:         make(map[string]SignerInfo),
		ThresholdConfig: thresholds,
		WeightByType:    make(map[SignatureType]uint32),
	}

	for _, signer := range signers {
		ctx.Signers[signer.SignerKey] = signer
	}

	t.accountContexts[accountID] = ctx
}

func (t *Tracker) RecordSignatureVerification(accountID, signerKey string, sigType SignatureType, verified bool, weight uint32) {
	status := "invalid"
	if verified {
		status = "valid"
	}

	t.RecordEvent(AuthEvent{
		EventType:     "signature_verification",
		AccountID:     accountID,
		SignerKey:     signerKey,
		SignatureType: sigType,
		Weight:        weight,
		Status:        status,
	})

	if !verified {
		return
	}

	t.mu.Lock()
	if ctx, ok := t.accountContexts[accountID]; ok {
		ctx.CollectedWeight += weight
		ctx.WeightByType[sigType] += weight
	}
	t.mu.Unlock()
}

func (t *Tracker) RecordThresholdCheck(accountID string, requiredWeight, collectedWeight uint32, passed bool) {
	details := ""
	if !passed {
		details = fmt.Sprintf("required %d, got %d", requiredWeight, collectedWeight)
		t.recordFailure(accountID, ReasonThresholdNotMet, requiredWeight, collectedWeight)
	}

	status := "passed"
	if !passed {
		status = "failed"
	}

	t.RecordEvent(AuthEvent{
		EventType: "threshold_check",
		AccountID: accountID,
		Status:    status,
		Details:   details,
	})
}

func (t *Tracker) RecordCustomContractCall(accountID, contractID, method string, params []string, result string, err error) {
	event := AuthEvent{
		EventType: "custom_contract_auth",
		AccountID: accountID,
		Status:    result,
		Details:   fmt.Sprintf("%s::%s", contractID, method),
	}

	if err != nil {
		event.ErrorReason = ReasonCustomContractFailed
	}

	t.RecordEvent(event)
}

func (t *Tracker) recordFailure(accountID string, reason AuthFailureReason, requiredWeight, collectedWeight uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()

	failure := AuthFailure{
		AccountID:       accountID,
		FailureReason:   reason,
		RequiredWeight:  requiredWeight,
		CollectedWeight: collectedWeight,
		MissingWeight:   requiredWeight - collectedWeight,
		FailedSigners:   make([]SignerInfo, 0),
	}

	if ctx, ok := t.accountContexts[accountID]; ok {
		failure.TotalSigners = uint32(len(ctx.Signers))
	}

	t.failures = append(t.failures, failure)
}

func (t *Tracker) GenerateTrace() *AuthTrace {
	t.mu.RLock()
	defer t.mu.RUnlock()

	trace := &AuthTrace{
		AuthEvents:       t.events,
		Failures:         t.failures,
		SignatureWeights: make([]KeyWeight, 0),
		CustomContracts:  make([]CustomContractAuth, 0),
	}

	if len(t.failures) == 0 {
		trace.Success = true
	}

	for _, event := range t.events {
		if event.EventType == "signature_verification" && event.Status == "valid" {
			trace.ValidSignatures++
		}
	}

	return trace
}

func (t *Tracker) GetFailureReport(accountID string) *AuthFailure {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, failure := range t.failures {
		if failure.AccountID == accountID {
			return &failure
		}
	}

	return nil
}

func (t *Tracker) GetAuthEvents(accountID string) []AuthEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var events []AuthEvent
	for _, event := range t.events {
		if event.AccountID == accountID {
			events = append(events, event)
		}
	}

	return events
}

func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.events = make([]AuthEvent, 0)
	t.failures = make([]AuthFailure, 0)
	t.accountContexts = make(map[string]*AccountAuthContext)
}
