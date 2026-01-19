package simulator

// SimulationRequest is the JSON object passed to the Rust binary via Stdin
type SimulationRequest struct {
	// XDR encoded TransactionEnvelope
	EnvelopeXdr string `json:"envelope_xdr"`
	// XDR encoded TransactionResultMeta (historical data)
	ResultMetaXdr string `json:"result_meta_xdr"`
	// XDR encoded LedgerHeader (optional, for context)
	// LedgerHeaderXdr string `json:"ledger_header_xdr,omitempty"`
}

// SimulationResponse is the JSON object returned by the Rust binary via Stdout
type SimulationResponse struct {
	Status string   `json:"status"` // "success" or "error"
	Error  string   `json:"error,omitempty"`
	Events []string `json:"events,omitempty"` // Diagnostic events
	Logs   []string `json:"logs,omitempty"`   // Host debug logs
}
