// Package rpc defines constants and shared types used when communicating
// with a Stellar JSON-RPC node (soroban-rpc / stellar-rpc).
//
// All RPC method names that the erst CLI sends over the wire are declared here
// as typed string constants so that no caller ever hard-codes a raw string
// literal.  Any future rename of a method only requires a change in this one
// file.
package rpc

// Method is the string type used for every JSON-RPC "method" field.
// Using a distinct type (rather than plain string) prevents accidental
// passing of arbitrary strings to functions that expect a valid RPC method.
type Method string

// Stellar JSON-RPC method constants.
//
// Names match the Stellar RPC specification exactly:
// https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods
const (
	// MethodGetTransaction fetches the result of a single submitted
	// transaction identified by its hash.
	MethodGetTransaction Method = "getTransaction"

	// MethodGetTransactions fetches details for multiple transactions over a
	// ledger range.
	MethodGetTransactions Method = "getTransactions"

	// MethodSendTransaction submits a signed transaction envelope to the
	// network for inclusion in a future ledger.
	MethodSendTransaction Method = "sendTransaction"

	// MethodSimulateTransaction performs a trial (dry-run) invocation of a
	// Soroban smart contract without modifying ledger state.
	MethodSimulateTransaction Method = "simulateTransaction"

	// MethodGetLedgerEntries reads live ledger state for one or more ledger
	// keys (accounts, contract data, WASM code, etc.).
	MethodGetLedgerEntries Method = "getLedgerEntries"

	// MethodGetLedgers returns full ledger metadata for a range of ledgers.
	MethodGetLedgers Method = "getLedgers"

	// MethodGetEvents queries for contract or system events emitted over a
	// ledger range, with optional topic filters.
	MethodGetEvents Method = "getEvents"

	// MethodGetLatestLedger returns the sequence number and hash of the most
	// recent ledger known to the RPC node.
	MethodGetLatestLedger Method = "getLatestLedger"

	// MethodGetNetwork returns the network passphrase and protocol version
	// reported by the RPC node.
	MethodGetNetwork Method = "getNetwork"

	// MethodGetVersionInfo returns the RPC node's build version, captive-core
	// version, and supported protocol version.
	MethodGetVersionInfo Method = "getVersionInfo"

	// MethodGetFeeStats returns fee statistics gathered by the RPC node for
	// recent ledgers.
	MethodGetFeeStats Method = "getFeeStats"

	// MethodHealth checks whether the RPC node considers itself healthy.
	MethodHealth Method = "getHealth"
)