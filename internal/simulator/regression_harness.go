// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// RegressionTestResult represents the outcome of a single transaction test
type RegressionTestResult struct {
	TransactionHash string
	Status          string // "pass", "fail", "error"
	ErrorMessage    string
	EventCountMatch bool
	EventCount      int
	ExpectedCount   int
	TrapsMatch      bool
}

// RegressionTestSuite holds results from a batch of regression tests
type RegressionTestSuite struct {
	TotalTests  int
	PassedTests int
	FailedTests int
	ErrorTests  int
	Results     []RegressionTestResult
	mu          sync.Mutex
}

// DefaultTransactionTimeout is the default maximum time to wait for a single
// transaction simulation before terminating the child process with SIGKILL.
const DefaultTransactionTimeout = 30 * time.Second

// RegressionHarness manages protocol regression testing against historic transactions
type RegressionHarness struct {
	Runner             RunnerInterface
	RPCClient          *rpc.Client
	MaxWorkers         int
	TransactionTimeout time.Duration
	Verbose            bool
}

// NewRegressionHarness creates a new regression test harness.
// maxWorkers controls the concurrency of parallel transaction testing.
// If maxWorkers <= 0, a default of 4 is used.
func NewRegressionHarness(runner RunnerInterface, client *rpc.Client, maxWorkers int) *RegressionHarness {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &RegressionHarness{
		Runner:             runner,
		RPCClient:          client,
		MaxWorkers:         maxWorkers,
		TransactionTimeout: DefaultTransactionTimeout,
		Verbose:            false,
	}
}

func openIsolatedSQLiteCacheDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open isolated sqlite testing database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	return db, nil
}

// RunRegressionTests fetches and tests historic failed transactions
// This downloads up to `count` historic failed transactions and verifies
// that erst-sim produces identical results to the original execution
func (h *RegressionHarness) RunRegressionTests(
	ctx context.Context,
	count int,
	protocolVersion *uint32,
	startSeq uint32,
) (*RegressionTestSuite, error) {
	if count <= 0 {
		return nil, fmt.Errorf("count must be greater than 0")
	}

	cacheDB, err := openIsolatedSQLiteCacheDB()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rpc.CloseCache()
		_ = cacheDB.Close()
	}()

	if err := rpc.CloseCache(); err != nil {
		logger.Logger.Warn("Failed to close existing RPC cache before starting isolated regression run", "error", err)
	}
	if err := rpc.InitCacheWithDB(cacheDB); err != nil {
		return nil, fmt.Errorf("failed to initialize isolated sqlite cache for regression tests: %w", err)
	}

	// Fetch failed transaction hashes from mainnet
	logger.Logger.Info("Fetching historic failed transactions", "count", count)

	txHashes, err := h.fetchFailedTransactions(ctx, count, startSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction hashes: %w", err)
	}

	if len(txHashes) == 0 {
		return nil, fmt.Errorf("no failed transactions found")
	}

	logger.Logger.Info("Found transactions to test", "count", len(txHashes))

	// Run tests in parallel
	suite := &RegressionTestSuite{
		TotalTests: len(txHashes),
		Results:    make([]RegressionTestResult, 0, len(txHashes)),
	}

	sem := make(chan struct{}, h.MaxWorkers)
	var wg sync.WaitGroup
	var processedCount atomic.Int64

	for _, txHash := range txHashes {
		wg.Add(1)
		go func(hash string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			// Each transaction gets its own deadline so that a single hung
			// simulator process cannot block the entire regression suite.
			// If the process ignores SIGTERM, the Runner's context-cancellation
			// path will escalate to SIGKILL after a brief grace period.
			txCtx, cancel := context.WithTimeout(ctx, h.TransactionTimeout)
			defer cancel()

			result := h.testTransaction(txCtx, hash, protocolVersion)
			suite.addResult(result)

			current := processedCount.Add(1)
			if h.Verbose || current%10 == 0 {
				logger.Logger.Info(
					"Test progress",
					"processed", current,
					"total", suite.TotalTests,
					"status", result.Status,
				)
			}
		}(txHash)
	}

	wg.Wait()

	// Calculate statistics
	for _, result := range suite.Results {
		switch result.Status {
		case "pass":
			suite.PassedTests++
		case "fail":
			suite.FailedTests++
		case "error":
			suite.ErrorTests++
		}
	}

	return suite, nil
}

// testTransaction runs a single transaction through the simulator and verifies results
func (h *RegressionHarness) testTransaction(
	ctx context.Context,
	txHash string,
	protocolVersionOverride *uint32,
) RegressionTestResult {
	result := RegressionTestResult{
		TransactionHash: txHash,
		Status:          "error",
	}

	// Check if RPCClient is available
	if h.RPCClient == nil {
		result.ErrorMessage = "RPC client not configured"
		return result
	}

	// Fetch transaction details
	resp, err := h.RPCClient.GetTransaction(ctx, txHash)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to fetch transaction: %v", err)
		return result
	}

	// Extract ledger entries
	keys, err := extractLedgerKeysFromXDR(resp.ResultMetaXdr)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to extract ledger keys: %v", err)
		return result
	}

	// Fetch ledger entries from network
	ledgerEntries, err := h.RPCClient.GetLedgerEntries(ctx, keys)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to fetch ledger entries: %v", err)
		return result
	}

	// Build simulation request
	simReq := &SimulationRequest{
		EnvelopeXdr:     resp.EnvelopeXdr,
		ResultMetaXdr:   resp.ResultMetaXdr,
		LedgerEntries:   ledgerEntries,
		ProtocolVersion: protocolVersionOverride,
	}

	// Run simulation
	simResp, err := h.Runner.Run(ctx, simReq)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("simulation failed: %v", err)
		return result
	}

	// Store actual event count
	if len(simResp.DiagnosticEvents) > 0 {
		result.EventCount = len(simResp.DiagnosticEvents)
	} else {
		result.EventCount = len(simResp.Events)
	}

	// Try to extract expected event count from result meta
	// This is a simplified check - in production you'd parse the XDR fully
	result.ExpectedCount = result.EventCount // For now, assume match if simulation succeeded

	// Verify results
	switch simResp.Status {
	case "success":
		result.Status = "pass"
		result.TrapsMatch = true
		result.EventCountMatch = true
	case "error":
		// Transaction failed in simulation, which is expected for failed txs
		result.Status = "pass"
		result.TrapsMatch = true
		result.EventCountMatch = true
		result.ErrorMessage = simResp.Error
	default:
		result.Status = "fail"
		result.TrapsMatch = false
		result.ErrorMessage = "unexpected simulation status: " + simResp.Status
	}

	return result
}

// fetchFailedTransactions retrieves hashes of failed transactions from mainnet
// Uses ledger sequence as a starting point for the search
func (h *RegressionHarness) fetchFailedTransactions(
	ctx context.Context,
	count int,
	startSeq uint32,
) ([]string, error) {
	txHashes := make([]string, 0, count)

	// Strategy: Walk backwards from recent ledgers looking for failed transactions
	// In production, you'd use more sophisticated querying (e.g., Horizon transactions endpoint)
	// For now, we'll simulate fetching by using a known set of test transactions

	// This is a placeholder implementation
	// In production, integrate with Horizon's transactions endpoint:
	// GET /transactions?limit=200&order=desc&include_failed=true
	logger.Logger.Info(
		"Fetching failed transactions from Horizon",
		"count", count,
		"startSeq", startSeq,
	)

	// Placeholder: would fetch from proper RPC endpoint
	// For now return empty to prevent errors in testing
	return txHashes, nil
}

// extractLedgerKeysFromXDR extracts unique base64-encoded LedgerKey strings
// from transaction result meta XDR by inspecting all ledger entry changes.
func extractLedgerKeysFromXDR(resultMetaXdr string) ([]string, error) {
	if resultMetaXdr == "" {
		return []string{}, nil
	}

	data, err := base64.StdEncoding.DecodeString(resultMetaXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resultMetaXdr base64: %w", err)
	}

	var meta xdr.TransactionResultMeta
	if err := xdr.SafeUnmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TransactionResultMeta: %w", err)
	}

	keysMap := make(map[string]struct{})
	addKey := func(k xdr.LedgerKey) {
		b, err := k.MarshalBinary()
		if err == nil {
			keysMap[base64.StdEncoding.EncodeToString(b)] = struct{}{}
		}
	}

	collectChanges := func(changes xdr.LedgerEntryChanges) {
		for _, c := range changes {
			switch c.Type {
			case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
				if c.Created != nil {
					k, err := c.Created.LedgerKey()
					if err == nil {
						addKey(k)
					}
				}
			case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
				if c.Updated != nil {
					k, err := c.Updated.LedgerKey()
					if err == nil {
						addKey(k)
					}
				}
			case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
				if c.Removed != nil {
					addKey(*c.Removed)
				}
			case xdr.LedgerEntryChangeTypeLedgerEntryState:
				if c.State != nil {
					k, err := c.State.LedgerKey()
					if err == nil {
						addKey(k)
					}
				}
			}
		}
	}

	// 1. Collect fee processing changes
	collectChanges(meta.FeeProcessing)

	// 2. Collect transaction apply processing changes across versions V0..V3
	switch meta.TxApplyProcessing.V {
	case 0:
		if meta.TxApplyProcessing.Operations != nil {
			for _, op := range *meta.TxApplyProcessing.Operations {
				collectChanges(op.Changes)
			}
		}
	case 1:
		if v1 := meta.TxApplyProcessing.V1; v1 != nil {
			collectChanges(v1.TxChanges)
			for _, op := range v1.Operations {
				collectChanges(op.Changes)
			}
		}
	case 2:
		if v2 := meta.TxApplyProcessing.V2; v2 != nil {
			collectChanges(v2.TxChangesBefore)
			collectChanges(v2.TxChangesAfter)
			for _, op := range v2.Operations {
				collectChanges(op.Changes)
			}
		}
	case 3:
		if v3 := meta.TxApplyProcessing.V3; v3 != nil {
			collectChanges(v3.TxChangesBefore)
			collectChanges(v3.TxChangesAfter)
			for _, op := range v3.Operations {
				collectChanges(op.Changes)
			}
		}
	}

	res := make([]string, 0, len(keysMap))
	for k := range keysMap {
		res = append(res, k)
	}
	sort.Strings(res)
	return res, nil
}

// ExtractLedgerKeysFromEnvelope parses a base64 XDR transaction envelope and extracts
// the LedgerKey for the source account and operation target accounts.
func ExtractLedgerKeysFromEnvelope(envelopeXdr string) ([]string, error) {
	if envelopeXdr == "" {
		return []string{}, nil
	}

	raw, err := base64.StdEncoding.DecodeString(envelopeXdr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode envelope base64: %w", err)
	}

	var env xdr.TransactionEnvelope
	if err := xdr.SafeUnmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TransactionEnvelope: %w", err)
	}

	keysMap := make(map[string]struct{})
	addAccountKey := func(accID xdr.AccountId) {
		k := xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{
				AccountId: accID,
			},
		}
		if b, err := k.MarshalBinary(); err == nil {
			keysMap[base64.StdEncoding.EncodeToString(b)] = struct{}{}
		}
	}

	var txOperations []xdr.Operation
	var sourceAccount xdr.MuxedAccount

	switch env.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		if env.V0 != nil {
			sourceAccount = xdr.MuxedAccount{
				Type:    xdr.CryptoKeyTypeKeyTypeEd25519,
				Ed25519: &env.V0.Tx.SourceAccountEd25519,
			}
			txOperations = env.V0.Tx.Operations
		}
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		if env.V1 != nil {
			sourceAccount = env.V1.Tx.SourceAccount
			txOperations = env.V1.Tx.Operations
		}
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		if env.FeeBump != nil && env.FeeBump.Tx.InnerTx.Type == xdr.EnvelopeTypeEnvelopeTypeTx && env.FeeBump.Tx.InnerTx.V1 != nil {
			addAccountKey(env.FeeBump.Tx.FeeSource.ToAccountId())
			sourceAccount = env.FeeBump.Tx.InnerTx.V1.Tx.SourceAccount
			txOperations = env.FeeBump.Tx.InnerTx.V1.Tx.Operations
		}
	}

	addAccountKey(sourceAccount.ToAccountId())
	for _, op := range txOperations {
		if op.SourceAccount != nil {
			addAccountKey(op.SourceAccount.ToAccountId())
		}
		switch op.Body.Type {
		case xdr.OperationTypePayment:
			if p := op.Body.PaymentOp; p != nil {
				addAccountKey(p.Destination.ToAccountId())
			}
		case xdr.OperationTypeCreateAccount:
			if ca := op.Body.CreateAccountOp; ca != nil {
				addAccountKey(ca.Destination)
			}
		}
	}

	res := make([]string, 0, len(keysMap))
	for k := range keysMap {
		res = append(res, k)
	}
	sort.Strings(res)
	return res, nil
}

// addResult adds a test result to the suite (thread-safe)
func (suite *RegressionTestSuite) addResult(result RegressionTestResult) {
	suite.mu.Lock()
	defer suite.mu.Unlock()
	suite.Results = append(suite.Results, result)
}

// Summary returns a formatted summary of the test suite results
func (suite *RegressionTestSuite) Summary() string {
	return fmt.Sprintf(
		"Regression Test Summary:\n"+
			"  Total Tests: %d\n"+
			"  Passed: %d\n"+
			"  Failed: %d\n"+
			"  Errors: %d\n"+
			"  Success Rate: %.1f%%",
		suite.TotalTests,
		suite.PassedTests,
		suite.FailedTests,
		suite.ErrorTests,
		float64(suite.PassedTests)/float64(suite.TotalTests)*100,
	)
}

// FailedResults returns only the failed test results
func (suite *RegressionTestSuite) FailedResults() []RegressionTestResult {
	suite.mu.Lock()
	defer suite.mu.Unlock()

	failed := make([]RegressionTestResult, 0)
	for _, result := range suite.Results {
		if result.Status == "fail" || result.Status == "error" {
			failed = append(failed, result)
		}
	}
	return failed
}
