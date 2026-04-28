// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/dotandev/hintents/internal/compare"
	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/visualizer"

	"github.com/spf13/cobra"
)

// ─── flags specific to the compare command ────────────────────────────────────

var (
	cmpNetworkFlag   string
	cmpRPCURLFlag    string
	cmpRPCTokenFlag  string
	cmpLocalWasmFlag string
	cmpBridgeWasm    []string
	cmpOptimizeFlag  bool
	cmpArgsFlag      []string
	cmpVerboseFlag   bool
	cmpSimPathFlag   string
	cmpThemeFlag     string
	cmpProtoFlag     uint32
)

// compareCmd implements `erst compare`.
var compareCmd = &cobra.Command{
	Use:     "compare <transaction-hash>",
	GroupID: "testing",
	Short:   "Compare replay: local WASM vs on-chain WASM side-by-side",
	Long: `Simultaneously replay a transaction against a local WASM file and the on-chain
contract, then display a side-by-side diff of events, diagnostic output, budget
usage, and divergent call paths.

This is the primary tool for "What broke when I updated my contract?" debugging.

How it works:
  1. Fetch the transaction envelope and ledger state from the network.
  2. Run two simulation passes in parallel:
       - Pass A: uses the local WASM file you provide (--wasm).
       - Pass B: uses the on-chain WASM (normal replay, no --wasm flag).
  3. Diff the two results and print a colour-coded side-by-side report.

Examples:
  # Compare your local contract against a mainnet transaction
  erst compare <tx-hash> --wasm ./my_contract.wasm

  # Compare on testnet with verbose output
  erst compare <tx-hash> --wasm ./contract.wasm --network testnet --verbose

  # Bridge external contract calls to local builds during replay
  erst compare <tx-hash> --wasm ./root.wasm --bridge-wasm CABC...=./deps/token.wasm

  # Override the protocol version used for both passes
  erst compare <tx-hash> --wasm ./contract.wasm --protocol-version 22`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if cmpLocalWasmFlag == "" {
			return errors.WrapValidationError("--wasm flag is required for compare mode")
		}
		if _, statErr := os.Stat(cmpLocalWasmFlag); os.IsNotExist(statErr) {
			return errors.WrapValidationError(fmt.Sprintf("WASM file not found: %s", cmpLocalWasmFlag))
		}
		if _, err := parseContractWasmOverrideSpecs(cmpBridgeWasm); err != nil {
			return errors.WrapValidationError(err.Error())
		}
		if validateErr := rpc.ValidateTransactionHash(args[0]); validateErr != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid transaction hash: %v", validateErr))
		}
		switch rpc.Network(cmpNetworkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			// valid
		default:
			return errors.WrapInvalidNetwork(cmpNetworkFlag)
		}
		return nil
	},
	RunE: runCompare,
}

func init() {
	compareCmd.Flags().StringVarP(&cmpNetworkFlag, "network", "n", string(rpc.Mainnet),
		"Stellar network (testnet, mainnet, futurenet)")
	compareCmd.Flags().StringVar(&cmpRPCURLFlag, "rpc-url", "",
		"Custom Soroban RPC URL")
	compareCmd.Flags().StringVar(&cmpRPCTokenFlag, "rpc-token", "",
		"RPC authentication token (or ERST_RPC_TOKEN env var)")
	compareCmd.Flags().StringVar(&cmpLocalWasmFlag, "wasm", "",
		"Path to local WASM file (required)")
	compareCmd.Flags().StringSliceVar(&cmpBridgeWasm, "bridge-wasm", nil,
		"Repeatable external contract override in the form <contract-id>=<path>")
	compareCmd.Flags().BoolVar(&cmpOptimizeFlag, "optimize", false,
		"Run dead-code elimination on local WASM before simulation")
	compareCmd.Flags().StringSliceVar(&cmpArgsFlag, "args", []string{},
		"Mock arguments to pass to the local WASM execution")
	compareCmd.Flags().BoolVarP(&cmpVerboseFlag, "verbose", "v", false,
		"Print full simulation JSON for both passes")
	compareCmd.Flags().StringVar(&cmpSimPathFlag, "sim-path", "",
		"Path to erst-sim binary (overrides auto-discovery)")
	compareCmd.Flags().StringVar(&cmpThemeFlag, "theme", "",
		"Colour theme (default, deuteranopia, protanopia, tritanopia, high-contrast)")
	compareCmd.Flags().Uint32Var(&cmpProtoFlag, "protocol-version", 0,
		"Override protocol version for both simulation passes (20, 21, 22, …)")
	_ = compareCmd.RegisterFlagCompletionFunc("network", completeNetworkFlag)
	_ = compareCmd.RegisterFlagCompletionFunc("theme", completeThemeFlag)
	rootCmd.AddCommand(compareCmd)
}

// ─── main handler ─────────────────────────────────────────────────────────────

func runCompare(cmd *cobra.Command, cmdArgs []string) error {
	ctx := cmd.Context()
	txHash := cmdArgs[0]
	localWasmPath := cmpLocalWasmFlag

	optimizedPath, report, cleanup, err := optimizeWasmFileIfRequested(localWasmPath, cmpOptimizeFlag)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to optimize local WASM: %v", err))
	}
	defer cleanup()
	localWasmPath = optimizedPath

	// Logging level
	if cmpVerboseFlag {
		logger.SetLevel(slog.LevelInfo)
	} else {
		logger.SetLevel(slog.LevelWarn)
	}

	// Theme
	if cmpThemeFlag != "" {
		visualizer.SetTheme(visualizer.Theme(cmpThemeFlag))
	} else {
		visualizer.SetTheme(visualizer.DetectTheme())
	}

	fmt.Printf("%s  Compare Replay\n", visualizer.Symbol("chart"))
	fmt.Printf("Transaction : %s\n", txHash)
	fmt.Printf("Network     : %s\n", cmpNetworkFlag)
	fmt.Printf("Local WASM  : %s\n", cmpLocalWasmFlag)
	if cmpOptimizeFlag {
		printOptimizationReport(report)
	}
	fmt.Println()

	// ── Build RPC client ────────────────────────────────────────────────────
	token := cmpRPCTokenFlag
	if token == "" {
		token = os.Getenv("ERST_RPC_TOKEN")
	}
	if token == "" {
		if cfg, cfgErr := config.Load(); cfgErr == nil && cfg.RPCToken != "" {
			token = cfg.RPCToken
		}
	}

	clientOpts := []rpc.ClientOption{
		rpc.WithNetwork(rpc.Network(cmpNetworkFlag)),
		rpc.WithToken(token),
	}
	if cmpRPCURLFlag != "" {
		urls := splitTrimmed(cmpRPCURLFlag)
		clientOpts = append(clientOpts, rpc.WithAltURLs(urls))
	} else {
		if cfg, cfgErr := config.Load(); cfgErr == nil {
			if len(cfg.RpcUrls) > 0 {
				clientOpts = append(clientOpts, rpc.WithAltURLs(cfg.RpcUrls))
			} else if cfg.RpcUrl != "" {
				clientOpts = append(clientOpts, rpc.WithHorizonURL(cfg.RpcUrl))
			}
			if cfg.FailureThreshold > 0 {
				clientOpts = append(clientOpts, rpc.WithCircuitBreakerThreshold(cfg.FailureThreshold))
			}
			if cfg.RetryTimeout > 0 {
				clientOpts = append(clientOpts, rpc.WithCircuitBreakerTimeout(cfg.RetryTimeout))
			}
		}
	}

	client, err := rpc.NewClient(clientOpts...)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to create RPC client: %v", err))
	}

	// ── Fetch transaction ───────────────────────────────────────────────────
	fmt.Printf("%s Fetching transaction from %s...\n", visualizer.Symbol("pin"), cmpNetworkFlag)
	txResp, err := client.GetTransaction(ctx, txHash)
	if err != nil {
		return errors.WrapRPCConnectionFailed(err)
	}
	fmt.Printf("%s Fetched (envelope: %d bytes)\n\n", visualizer.Success(), len(txResp.EnvelopeXdr))

	// ── Extract ledger keys & entries ───────────────────────────────────────
	keys, err := extractLedgerKeys(txResp.ResultMetaXdr)
	if err != nil {
		return errors.WrapUnmarshalFailed(err, "result meta")
	}

	ledgerEntries, err := rpc.ExtractLedgerEntriesFromMeta(txResp.ResultMetaXdr)
	if err != nil {
		logger.Logger.Warn("Falling back to live ledger entry fetch", "error", err)
		ledgerEntries, err = client.GetLedgerEntries(ctx, keys)
		if err != nil {
			return errors.WrapRPCConnectionFailed(err)
		}
	}

	// ── Build simulator runner ───────────────────────────────────────────────
	runner, err := simulator.NewRunner(cmpSimPathFlag, cmpVerboseFlag)
	if err != nil {
		return errors.WrapSimulatorNotFound(err.Error())
	}

	overridePaths, err := parseContractWasmOverrideSpecs(cmpBridgeWasm)
	if err != nil {
		return errors.WrapValidationError(err.Error())
	}
	targetContractID, err := getContractIDFromEnvelope(txResp.EnvelopeXdr)
	if err != nil {
		return errors.WrapSimulationLogicError(fmt.Sprintf("failed to identify contract from transaction: %v", err))
	}
	overridePaths[fmt.Sprintf("%x", *targetContractID)] = localWasmPath

	replayLedgerEntries, err := prepareReplayLedgerEntries(ctx, client, ledgerEntries, overridePaths)
	if err != nil {
		return errors.WrapSimulationLogicError(fmt.Sprintf("failed to prepare replay state: %v", err))
	}

	localLedgerEntries, err := applyReplayLedgerEntryOverrides(replayLedgerEntries, overridePaths)
	if err != nil {
		return errors.WrapSimulationLogicError(fmt.Sprintf("failed to prepare replay overrides: %v", err))
	}

	// ── Run two simulation passes in parallel ────────────────────────────────
	fmt.Printf("%s Running two simulation passes in parallel...\n", visualizer.Symbol("play"))
	fmt.Printf("   Pass A – local WASM  : %s\n", localWasmPath)
	fmt.Printf("   Pass B – on-chain WASM: (using network ledger state)\n\n")

	localResult, onChainResult, runErr := runBothPasses(ctx, runner, txResp, localLedgerEntries, replayLedgerEntries, localWasmPath)
	if runErr != nil {
		return runErr
	}

	if cmpVerboseFlag {
		printVerboseResponse("LOCAL WASM", localResult)
		printVerboseResponse("ON-CHAIN WASM", onChainResult)
	}

	// ── Diff & render ────────────────────────────────────────────────────────
	diffResult := compare.Diff(localResult, onChainResult)
	compare.Render(diffResult)

	return nil
}

// runBothPasses executes the local and on-chain simulation concurrently.
func runBothPasses(
	ctx context.Context,
	runner *simulator.Runner,
	txResp *rpc.TransactionResponse,
	localLedgerEntries map[string]string,
	ledgerEntries map[string]string,
	localWasmPath string,
) (localResult, onChainResult *simulator.SimulationResponse, err error) {
	var wg sync.WaitGroup
	var localErr, onChainErr error

	wg.Add(2)

	// Pass A – local WASM
	go func() {
		defer wg.Done()
		req := buildSimRequest(txResp, localLedgerEntries, &localWasmPath, cmpArgsFlag)
		localResult, localErr = runner.Run(ctx, req)
	}()

	// Pass B – on-chain (no --wasm flag, uses whatever is in the ledger)
	go func() {
		defer wg.Done()
		req := buildSimRequest(txResp, ledgerEntries, nil, nil)
		onChainResult, onChainErr = runner.Run(ctx, req)
	}()

	wg.Wait()

	if localErr != nil {
		return nil, nil, fmt.Errorf("local WASM simulation failed: %w", localErr)
	}
	if onChainErr != nil {
		return nil, nil, fmt.Errorf("on-chain simulation failed: %w", onChainErr)
	}
	return localResult, onChainResult, nil
}

// buildSimRequest constructs a SimulationRequest with optional local WASM override.
func buildSimRequest(
	txResp *rpc.TransactionResponse,
	ledgerEntries map[string]string,
	wasmPath *string,
	mockArgs []string,
) *simulator.SimulationRequest {
	req := &simulator.SimulationRequest{
		EnvelopeXdr:   txResp.EnvelopeXdr,
		ResultMetaXdr: txResp.ResultMetaXdr,
		LedgerEntries: ledgerEntries,
	}
	if wasmPath != nil && *wasmPath != "" {
		req.WasmPath = wasmPath
	}
	if len(mockArgs) > 0 {
		req.MockArgs = &mockArgs
	}
	if cmpProtoFlag > 0 {
		if validateErr := simulator.Validate(cmpProtoFlag); validateErr == nil {
			req.ProtocolVersion = &cmpProtoFlag
		}
	}
	return req
}

// printVerboseResponse prints the full simulation JSON for a named pass.
func printVerboseResponse(label string, resp *simulator.SimulationResponse) {
	fmt.Printf("\n──── VERBOSE: %s ────\n", label)
	fmt.Printf("  Status : %s\n", resp.Status)
	if resp.Error != "" {
		fmt.Printf("  Error  : %s\n", resp.Error)
	}
	fmt.Printf("  Events : %d\n", len(resp.Events))
	fmt.Printf("  DiagEvt: %d\n", len(resp.DiagnosticEvents))
	for _, e := range resp.Events {
		fmt.Printf("    • %s\n", e)
	}
	if resp.BudgetUsage != nil {
		b := resp.BudgetUsage
		fmt.Printf("  Budget : CPU=%d  Mem=%d  Ops=%d\n",
			b.CPUInstructions, b.MemoryBytes, b.OperationsCount)
	}
	fmt.Println()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func parseContractWasmOverrideSpecs(specs []string) (map[string]string, error) {
	overrides := make(map[string]string, len(specs))
	for _, spec := range specs {
		parts := strings.SplitN(spec, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --bridge-wasm value %q: expected <contract-id>=<path>", spec)
		}

		contractID := strings.TrimSpace(parts[0])
		wasmPath := strings.TrimSpace(parts[1])
		if contractID == "" || wasmPath == "" {
			return nil, fmt.Errorf("invalid --bridge-wasm value %q: contract id and path are required", spec)
		}
		if _, err := os.Stat(wasmPath); err != nil {
			return nil, fmt.Errorf("bridge WASM file not found for %s: %w", contractID, err)
		}

		overrides[contractID] = wasmPath
	}
	return overrides, nil
}

func prepareReplayLedgerEntries(
	ctx context.Context,
	client *rpc.Client,
	baseEntries map[string]string,
	overridePaths map[string]string,
) (map[string]string, error) {
	entries := cloneStringMap(baseEntries)
	if len(overridePaths) == 0 {
		return entries, nil
	}

	contractIDs := make([]string, 0, len(overridePaths))
	for contractID := range overridePaths {
		contractIDs = append(contractIDs, contractID)
	}

	var err error
	entries, err = rpc.FetchBytecodeForTraceContractCalls(ctx, client, contractIDs, entries)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func applyReplayLedgerEntryOverrides(
	baseEntries map[string]string,
	overridePaths map[string]string,
) (map[string]string, error) {
	entries := cloneStringMap(baseEntries)
	for contractID, wasmPath := range overridePaths {
		wasmBytes, readErr := os.ReadFile(wasmPath)
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", wasmPath, readErr)
		}
		if _, applyErr := rpc.ApplyWasmOverrideToLedgerEntries(entries, contractID, wasmBytes); applyErr != nil {
			return nil, fmt.Errorf("apply override for %s: %w", contractID, applyErr)
		}
	}

	return entries, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
