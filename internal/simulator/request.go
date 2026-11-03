// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

type SimulationRequest struct {
	EnvelopeXdr    string            `json:"envelope_xdr"`
	ResultMetaXdr  string            `json:"result_meta_xdr"`
	LedgerEntries  map[string]string `json:"ledger_entries,omitempty"`
	ControlCommand string            `json:"control_command,omitempty"`
	RewindStep     *int              `json:"rewind_step,omitempty"`
	ForkParams     map[string]string `json:"fork_params,omitempty"`
	HarnessReset   bool              `json:"harness_reset,omitempty"`
	// Timestamp is seconds since the Unix epoch at which to run the simulation,
	// and maps to the simulator's `ledger_timestamp`, which seeds its clock.
	// The simulator also declares a `timestamp` field, but that one is a String
	// it never consumes, so sending a number there fails the whole request with
	// "invalid type: integer, expected a string".
	Timestamp        int64     `json:"ledger_timestamp,omitempty"`
	LedgerSequence   uint32    `json:"ledger_sequence,omitempty"`
	WasmPath         *string   `json:"wasm_path,omitempty"`
	NoCache          bool      `json:"no_cache,omitempty"`
	MockArgs         *[]string `json:"mock_args,omitempty"`
	Profile          bool      `json:"profile,omitempty"`
	ProtocolVersion  *uint32   `json:"protocol_version,omitempty"`
	MockBaseFee      *uint32   `json:"mock_base_fee,omitempty"`
	MockGasPrice     *uint64   `json:"mock_gas_price,omitempty"`
	MemoryLimit      *uint64   `json:"memory_limit,omitempty"`
	EnableCoverage   bool      `json:"enable_coverage,omitempty"`
	CoverageLCOVPath *string   `json:"coverage_lcov_path,omitempty"`
	// No omitempty: the Rust simulator's SimulationRequest declares this as a
	// bare (non-Option, non-#[serde(default)]) field, so it must always be
	// present in the JSON — omitempty would drop it when false and the
	// simulator would reject the request with "missing field
	// `enable_optimization_advisor`".
	EnableOptimizationAdvisor bool                   `json:"enable_optimization_advisor"`
	EnableSnapshots           bool                   `json:"enable_snapshots"`
	RestorePreamble           map[string]interface{} `json:"restore_preamble,omitempty"`
	AuthTraceOpts             *AuthTraceOptions      `json:"auth_trace_opts,omitempty"`
	CustomAuthCfg             map[string]interface{} `json:"custom_auth_config,omitempty"`
	ResourceCalibration       *ResourceCalibration   `json:"resource_calibration,omitempty"`

	SandboxNativeTokenCapStroops *uint64 `json:"sandbox_native_token_cap_stroops,omitempty"`
	ContractWasm                 *string `json:"contract_wasm,omitempty"`
	EnableAssetSafety            bool    `json:"enable_asset_safety,omitempty"`
}

type ResourceCalibration struct {
	SHA256Fixed      uint64 `json:"sha256_fixed"`
	SHA256PerByte    uint64 `json:"sha256_per_byte"`
	Keccak256Fixed   uint64 `json:"keccak256_fixed"`
	Keccak256PerByte uint64 `json:"keccak256_per_byte"`
	Ed25519Fixed     uint64 `json:"ed25519_fixed"`
}

type AuthTraceOptions struct {
	Enabled              bool `json:"enabled"`
	TraceCustomContracts bool `json:"trace_custom_contracts"`
	CaptureSigDetails    bool `json:"capture_sig_details"`
	MaxEventDepth        int  `json:"max_event_depth,omitempty"`
}
