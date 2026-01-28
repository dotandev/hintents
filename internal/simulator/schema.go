package simulator

import (
	"github.com/dotandev/hintents/internal/authtrace"
)

type SimulationRequest struct {
	EnvelopeXdr   string                 `json:"envelope_xdr"`
	ResultMetaXdr string                 `json:"result_meta_xdr"`
	LedgerEntries map[string]string      `json:"ledger_entries,omitempty"`
	AuthTraceOpts *AuthTraceOptions      `json:"auth_trace_opts,omitempty"`
	CustomAuthCfg map[string]interface{} `json:"custom_auth_config,omitempty"`
}

type AuthTraceOptions struct {
	Enabled              bool `json:"enabled"`
	TraceCustomContracts bool `json:"trace_custom_contracts"`
	CaptureSigDetails    bool `json:"capture_sig_details"`
	MaxEventDepth        int  `json:"max_event_depth,omitempty"`
}

type SimulationResponse struct {
	Status    string               `json:"status"`
	Error     string               `json:"error,omitempty"`
	Events    []string             `json:"events,omitempty"`
	Logs      []string             `json:"logs,omitempty"`
	AuthTrace *authtrace.AuthTrace `json:"auth_trace,omitempty"`
}
