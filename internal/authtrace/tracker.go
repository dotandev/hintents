// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package authtrace

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// knownSACContracts maps well-known Stellar Asset Contract IDs to their human-readable asset names.
// These are the built-in SAC contracts that wrap Stellar classic assets on Soroban.
var knownSACContracts = map[string]string{
	"CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC": "XLM (Native)",
	"CBIELTK6YBZJU5UP2WWQEQPMVNE6BEPVVD47VA5GFDSQLBPZS2FSPJVB": "USDC (Circle)",
}

// sacMethodSignatures lists method names commonly found in Stellar Asset Contracts.
var sacMethodSignatures = []string{
	"transfer",
	"transfer_from",
	"approve",
	"allowance",
	"balance",
	"mint",
	"burn",
	"burn_from",
	"clawback",
	"set_admin",
	"admin",
	"decimals",
	"name",
	"symbol",
	"total_supply",
	"authorized",
	"set_authorized",
}

type Tracker struct {
	mu              sync.RWMutex
	events          []AuthEvent
	failures        []AuthFailure
	config          Config
	accountContexts map[string]*AccountAuthContext
	// seenNonces tracks nonces per account to detect replay attacks (#1217).
	seenNonces map[string]map[string]int64
}

type AccountAuthContext struct {
	AccountID       string
	Signers         map[string]SignerInfo
	ThresholdConfig ThresholdConfig
	CollectedWeight uint32
	WeightByType    map[SignatureType]uint32
}

func NewTracker(config Config) *Tracker {
	return &Tracker{
		events:          make([]AuthEvent, 0),
		failures:        make([]AuthFailure, 0),
		config:          config,
		accountContexts: make(map[string]*AccountAuthContext),
		seenNonces:      make(map[string]map[string]int64),
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
	details := fmt.Sprintf("%s::%s", contractID, method)
	if len(params) > 0 {
		details = fmt.Sprintf("%s params=%v", details, params)
	}

	event := AuthEvent{
		EventType: "custom_contract_auth",
		AccountID: accountID,
		Status:    result,
		Details:   details,
	}

	if err != nil {
		event.ErrorReason = ReasonCustomContractFailed
	}

	t.RecordEvent(event)
}

// CheckReplayAttack detects common authorization anti-patterns that indicate a replay
// attack vulnerability (#1217):
//   - Missing or empty nonce: allows re-submission of the same signed payload.
//   - Duplicate nonce for the same account: the nonce was already consumed.
//   - Nonce timestamp too far in the past (>5 min): possible replay of an old message.
//
// When a vulnerability is detected the method records a ReasonReplayAttackDetected
// failure event and returns a non-nil ReplayAttackWarning describing the issue.
func (t *Tracker) CheckReplayAttack(accountID, nonce string, nonceTimestampMs int64) *ReplayAttackWarning {
	const staleThresholdMs = 5 * 60 * 1000 // 5 minutes

	now := time.Now().UnixMilli()

	// Anti-pattern 1: missing nonce.
	if strings.TrimSpace(nonce) == "" {
		warning := &ReplayAttackWarning{
			AccountID:   accountID,
			AntiPattern: "missing_nonce",
			Detail:      "no nonce present in auth payload; replay attacks are possible",
			DetectedAt:  now,
		}
		t.recordReplayEvent(accountID, warning)
		return warning
	}

	// Anti-pattern 2: stale timestamp.
	if nonceTimestampMs > 0 && now-nonceTimestampMs > staleThresholdMs {
		warning := &ReplayAttackWarning{
			AccountID:   accountID,
			AntiPattern: "stale_nonce_timestamp",
			Detail:      fmt.Sprintf("nonce timestamp is %dms old (threshold: %dms)", now-nonceTimestampMs, staleThresholdMs),
			DetectedAt:  now,
		}
		t.recordReplayEvent(accountID, warning)
		return warning
	}

	// Anti-pattern 3: duplicate nonce.
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.seenNonces[accountID] == nil {
		t.seenNonces[accountID] = make(map[string]int64)
	}

	if firstSeen, dup := t.seenNonces[accountID][nonce]; dup {
		warning := &ReplayAttackWarning{
			AccountID:   accountID,
			AntiPattern: "duplicate_nonce",
			Detail:      fmt.Sprintf("nonce %q was already used at %d", nonce, firstSeen),
			DetectedAt:  now,
		}
		// Record event without holding the lock a second time.
		event := AuthEvent{
			Timestamp:   now,
			EventType:   "replay_attack_detected",
			AccountID:   accountID,
			Status:      "failed",
			ErrorReason: ReasonReplayAttackDetected,
			Details:     warning.Detail,
		}
		if t.config.MaxEventDepth == 0 || len(t.events) < t.config.MaxEventDepth {
			t.events = append(t.events, event)
		}
		return warning
	}

	// Nonce is fresh and unseen — record it.
	t.seenNonces[accountID][nonce] = now
	return nil
}

// recordReplayEvent appends a replay-attack event without requiring the mutex
// (callers of CheckReplayAttack that do not hold the lock use RecordEvent which
// acquires the lock internally).
func (t *Tracker) recordReplayEvent(accountID string, w *ReplayAttackWarning) {
	t.RecordEvent(AuthEvent{
		Timestamp:   w.DetectedAt,
		EventType:   "replay_attack_detected",
		AccountID:   accountID,
		Status:      "failed",
		ErrorReason: ReasonReplayAttackDetected,
		Details:     fmt.Sprintf("[%s] %s", w.AntiPattern, w.Detail),
	})
}

// isSACContract returns true when contractID is a known Stellar Asset Contract
// or when the method name matches the SAC interface.
func isSACContract(contractID, method string) (bool, string) {
	if label, ok := knownSACContracts[contractID]; ok {
		return true, label
	}
	for _, sig := range sacMethodSignatures {
		if strings.EqualFold(method, sig) {
			return true, "unknown SAC asset"
		}
	}
	return false, ""
}

// RecordSACCall identifies and records a call to a Stellar Asset Contract (#1210).
// If the contractID or method matches the SAC interface the event is tagged with
// event_type "sac_call" so downstream consumers can distinguish SAC interactions
// from arbitrary custom contract calls. A SACCall value is constructed and
// serialised into the event Details for full auditability.
func (t *Tracker) RecordSACCall(accountID, contractID, method string, params []string, result string, err error) {
	isSAC, assetLabel := isSACContract(contractID, method)

	eventType := "custom_contract_auth"
	details := fmt.Sprintf("%s::%s", contractID, method)

	if isSAC {
		eventType = "sac_call"
		details = fmt.Sprintf("SAC[%s] %s::%s", assetLabel, contractID, method)
	}

	sac := SACCall{
		ContractID: contractID,
		AssetLabel: assetLabel,
		Method:     method,
		Params:     params,
		Result:     result,
	}
	if err != nil {
		sac.ErrorMsg = err.Error()
	}

	// Append serialised SACCall to details for full auditability.
	if encoded, jsonErr := json.Marshal(sac); jsonErr == nil {
		details = fmt.Sprintf("%s %s", details, encoded)
	}

	event := AuthEvent{
		EventType: eventType,
		AccountID: accountID,
		Status:    result,
		Details:   details,
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
	t.seenNonces = make(map[string]map[string]int64)
}

// ExportTraceJSON serialises the current AuthTrace to indented JSON suitable
// for ingestion by external audit tools (#1213). It delegates to the
// AuthTrace.ToJSON helper defined in types.go.
func (t *Tracker) ExportTraceJSON() ([]byte, error) {
	trace := t.GenerateTrace()
	out, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("authtrace: failed to marshal trace to JSON: %w", err)
	}
	return out, nil
}

// VerifyInvocationTreeIntegrity compares invocation tree nodes with execution
// trace nodes in order and reports precise structural mismatches.
func (t *Tracker) VerifyInvocationTreeIntegrity(invocationTree []string, executionTrace []string) error {
	maxLen := len(invocationTree)
	if len(executionTrace) > maxLen {
		maxLen = len(executionTrace)
	}

	for i := 0; i < maxLen; i++ {
		treeNodeExists := i < len(invocationTree)
		traceNodeExists := i < len(executionTrace)

		switch {
		case treeNodeExists && !traceNodeExists:
			err := fmt.Errorf("invocation tree mismatch: missing execution node at position %d (expected %q)", i, invocationTree[i])
			t.RecordEvent(AuthEvent{
				EventType:   "invocation_tree_integrity",
				AccountID:   "",
				Status:      "warning",
				ErrorReason: ReasonUnknown,
				Details:     err.Error(),
			})
			return err
		case !treeNodeExists && traceNodeExists:
			err := fmt.Errorf("invocation tree mismatch: unexpected execution node at position %d (actual %q)", i, executionTrace[i])
			t.RecordEvent(AuthEvent{
				EventType:   "invocation_tree_integrity",
				AccountID:   "",
				Status:      "warning",
				ErrorReason: ReasonUnknown,
				Details:     err.Error(),
			})
			return err
		case treeNodeExists && traceNodeExists && invocationTree[i] != executionTrace[i]:
			err := fmt.Errorf(
				"invocation tree mismatch: ordering/content mismatch at position %d (expected %q, got %q)",
				i,
				invocationTree[i],
				executionTrace[i],
			)
			t.RecordEvent(AuthEvent{
				EventType:   "invocation_tree_integrity",
				AccountID:   "",
				Status:      "warning",
				ErrorReason: ReasonUnknown,
				Details:     err.Error(),
			})
			return err
		}
	}

	return nil
}
