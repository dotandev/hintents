package simulator

// SimulationRequest is the JSON object passed to the Rust binary via Stdin
type SimulationRequest struct {
	// XDR encoded TransactionEnvelope
	EnvelopeXdr string `json:"envelope_xdr"`
	// XDR encoded TransactionResultMeta (historical data)
	ResultMetaXdr string `json:"result_meta_xdr"`
	// Snapshot of Ledger Entries (Key XDR -> Entry XDR) necessary for replay
	LedgerEntries map[string]string `json:"ledger_entries,omitempty"`
	// Enable profiling
	Profile bool `json:"profile,omitempty"`
}

// SimulationResponse is the JSON object returned by the Rust binary via Stdout
type SimulationResponse struct {
	Status string   `json:"status"` // "success" or "error"
	Error  string   `json:"error,omitempty"`
	Events []string `json:"events,omitempty"`     // Diagnostic events
	Logs   []string `json:"logs,omitempty"`       // Host debug logs
	Flamegraph string `json:"flamegraph,omitempty"` // SVG flamegraph
}
