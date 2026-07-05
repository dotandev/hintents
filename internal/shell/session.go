// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// maxShellTimestamp is the latest allowed ledger timestamp (2100-01-01 UTC).
// Prevents unbounded growth when the clock is behind during extended time-travel debugging.
const maxShellTimestamp int64 = 4102444800

// Session represents an interactive shell session with persistent ledger state
type Session struct {
	runner          simulator.RunnerInterface
	rpcClient       *rpc.Client
	network         rpc.Network
	ledgerEntries   map[string]string
	ledgerSequence  uint32
	timestamp       int64
	invocationCount int
	initialState    *LedgerState
}

// LedgerState represents the state of the ledger at a point in time
type LedgerState struct {
	Entries        map[string]string `json:"entries"`
	LedgerSequence uint32            `json:"ledger_sequence"`
	Timestamp      int64             `json:"timestamp"`
}

// StateSummary provides a summary of the current ledger state
type StateSummary struct {
	EntryCount      int
	LedgerSequence  uint32
	Timestamp       int64
	InvocationCount int
}

// InvocationResult represents the result of a contract invocation
type InvocationResult struct {
	Status string
	Error  string
	Events []string
	Logs   []string
}

// NewSession creates a new interactive shell session
func NewSession(runner simulator.RunnerInterface, rpcClient *rpc.Client, network rpc.Network) *Session {
	now := time.Now().Unix()
	return &Session{
		runner:         runner,
		rpcClient:      rpcClient,
		network:        network,
		ledgerEntries:  make(map[string]string),
		ledgerSequence: 1,
		timestamp:      now,
		initialState: &LedgerState{
			Entries:        make(map[string]string),
			LedgerSequence: 1,
			Timestamp:      now,
		},
	}
}

// Invoke executes a contract function and updates the ledger state
func (s *Session) Invoke(ctx context.Context, contractID, function string, args []string) (*InvocationResult, error) {
	// Build transaction envelope for the invocation
	envelopeXDR, err := s.buildInvocationEnvelope(contractID, function, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build envelope: %w", err)
	}

	// Create simulation request with snapshots enabled so updateLedgerState
	// can read post-execution ledger entry diffs from the response.
	req := &simulator.SimulationRequest{
		EnvelopeXdr:     envelopeXDR,
		ResultMetaXdr:   "",
		LedgerEntries:   s.ledgerEntries,
		Timestamp:       s.timestamp,
		LedgerSequence:  s.ledgerSequence,
		EnableSnapshots: true,
	}

	// Execute simulation
	resp, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("simulation failed: %w", err)
	}

	// Update ledger state based on simulation result
	s.updateLedgerState(resp)
	s.invocationCount++

	// Convert response to invocation result
	result := &InvocationResult{
		Status: resp.Status,
		Error:  resp.Error,
		Events: resp.Events,
		Logs:   resp.Logs,
	}

	return result, nil
}

// buildInvocationEnvelope creates a transaction envelope for contract invocation
func (s *Session) buildInvocationEnvelope(contractID, function string, args []string) (string, error) {
	// Decode contract ID from strkey (C...) or 32-byte hex
	var cid xdr.ContractId
	if len(contractID) > 0 && contractID[0] == 'C' {
		decoded, err := strkey.Decode(strkey.VersionByteContract, contractID)
		if err != nil {
			return "", fmt.Errorf("decode contract id: %w", err)
		}
		if len(decoded) != 32 {
			return "", fmt.Errorf("contract id must be 32 bytes, got %d", len(decoded))
		}
		copy(cid[:], decoded)
	} else {
		return "", fmt.Errorf("contract id must be a strkey C... address")
	}

	// Build ScVal arguments from string representations
	scArgs := make([]xdr.ScVal, 0, len(args))
	for _, arg := range args {
		s := xdr.ScString(arg)
		scArgs = append(scArgs, xdr.ScVal{
			Type: xdr.ScValTypeScvString,
			Str:  &s,
		})
	}

	// Build InvokeHostFunction operation
	fnName := xdr.ScSymbol(function)
	op := xdr.Operation{
		Body: xdr.OperationBody{
			Type: xdr.OperationTypeInvokeHostFunction,
			InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
				HostFunction: xdr.HostFunction{
					Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
					InvokeContract: &xdr.InvokeContractArgs{
						ContractAddress: xdr.ScAddress{
							Type:       xdr.ScAddressTypeScAddressTypeContract,
							ContractId: &cid,
						},
						FunctionName: fnName,
						Args:         scArgs,
					},
				},
			},
		},
	}

	// Use zero-bytes source account (placeholder for simulation — not submitted to network)
	var sourceBytes [32]byte
	sourceAccount := xdr.MuxedAccount{
		Type: xdr.CryptoKeyTypeKeyTypeEd25519,
		Ed25519: func() *xdr.Uint256 {
			u := xdr.Uint256(sourceBytes)
			return &u
		}(),
	}

	tx := xdr.Transaction{
		SourceAccount: sourceAccount,
		Fee:           100,
		SeqNum:        1,
		Cond:          xdr.Preconditions{Type: xdr.PreconditionTypePrecondNone},
		Memo:          xdr.Memo{Type: xdr.MemoTypeMemoNone},
		Operations:    []xdr.Operation{op},
		Ext:           xdr.TransactionExt{V: 0},
	}

	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx:         tx,
			Signatures: []xdr.DecoratedSignature{},
		},
	}

	envBytes, err := envelope.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal envelope: %w", err)
	}

	return base64.StdEncoding.EncodeToString(envBytes), nil
}

// updateLedgerState updates the session's ledger state based on simulation results.
// It parses ledger entry diffs from the simulator's inline snapshot payload and
// merges them into s.ledgerEntries to maintain persistent shell session state.
func (s *Session) updateLedgerState(resp *simulator.SimulationResponse) {
	s.ledgerSequence++

	now := time.Now().Unix()
	if now <= s.timestamp {
		now = s.timestamp + 1
	}
	if now > maxShellTimestamp {
		now = maxShellTimestamp
	}
	s.timestamp = now

	if resp.OptimizationReport == nil || resp.OptimizationReport.Snapshots == nil {
		return
	}

	// Each inline snapshot contains ledger entry pairs: [keyXDR_b64, entryXDR_b64].
	// Merge all snapshots so the session reflects the post-simulation state.
	for _, snapshot := range resp.OptimizationReport.Snapshots.Inline {
		for _, pair := range snapshot.LedgerEntries {
			if len(pair) == 2 {
				s.ledgerEntries[pair[0]] = pair[1]
			}
		}
	}
}

// GetStateSummary returns a summary of the current ledger state
func (s *Session) GetStateSummary() StateSummary {
	return StateSummary{
		EntryCount:      len(s.ledgerEntries),
		LedgerSequence:  s.ledgerSequence,
		Timestamp:       s.timestamp,
		InvocationCount: s.invocationCount,
	}
}

// SaveState saves the current ledger state to a file
func (s *Session) SaveState(filename string) error {
	state := &LedgerState{
		Entries:        s.ledgerEntries,
		LedgerSequence: s.ledgerSequence,
		Timestamp:      s.timestamp,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// LoadState loads ledger state from a file
func (s *Session) LoadState(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var state LedgerState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Update session state
	s.ledgerEntries = state.Entries
	s.ledgerSequence = state.LedgerSequence
	s.timestamp = state.Timestamp

	// Save as initial state for reset
	s.initialState = &state

	return nil
}

// ResetState resets the ledger state to the initial state
func (s *Session) ResetState() {
	s.ledgerEntries = make(map[string]string)
	for k, v := range s.initialState.Entries {
		s.ledgerEntries[k] = v
	}
	s.ledgerSequence = s.initialState.LedgerSequence
	s.timestamp = s.initialState.Timestamp
	s.invocationCount = 0
}
