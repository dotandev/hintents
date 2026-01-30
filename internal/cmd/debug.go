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

package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/localization"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/security"
	"github.com/dotandev/hintents/internal/session"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/dotandev/hintents/internal/tokenflow"

	"github.com/spf13/cobra"
	"github.com/stellar/go/xdr"
	"go.opentelemetry.io/otel/attribute"
)

var (
	networkFlag        string
	rpcURLFlag         string
	rpcTokenFlag       string
	tracingEnabled     bool
	otlpExporterURL    string
	generateTrace      bool
	traceOutputFile    string
	snapshotFlag       string
	compareNetworkFlag string
	verbose            bool
	wasmPath           string
	args               []string
)

// DebugCommand holds dependencies for the debug command
type DebugCommand struct {
	Runner simulator.RunnerInterface
}

// NewDebugCommand creates a new debug command with dependencies
func NewDebugCommand(runner simulator.RunnerInterface) *cobra.Command {
	debugCmd := &DebugCommand{Runner: runner}
	return debugCmd.createCommand()
}

func (d *DebugCommand) createCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug <transaction-hash>",
		Short: "Debug a failed Soroban transaction",
		Long: `Fetch a transaction envelope from the Stellar network and prepare it for simulation.

Example:
  erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
  erst debug --network testnet <tx-hash>`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate network flag
			switch rpc.Network(networkFlag) {
			case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
				return nil
			default:
				return fmt.Errorf("invalid network: %s. Must be one of: testnet, mainnet, futurenet", networkFlag)
			}
		},
		RunE: d.runDebug,
	}

	// Set up flags
	cmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	cmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	cmd.Flags().StringVar(&rpcTokenFlag, "rpc-token", "", "RPC authentication token (can also use ERST_RPC_TOKEN env var)")

	return cmd
}

func (d *DebugCommand) runDebug(cmd *cobra.Command, args []string) error {
	txHash := args[0]

	var client *rpc.Client
	if rpcURLFlag != "" {
		client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag), rpcTokenFlag)
	} else {
		client = rpc.NewClient(rpc.Network(networkFlag), rpcTokenFlag)
	}

	fmt.Printf("Debugging transaction: %s\n", txHash)
	fmt.Printf("Network: %s\n", networkFlag)
	if rpcURLFlag != "" {
		fmt.Printf("RPC URL: %s\n", rpcURLFlag)
	}

	// Fetch transaction details
	resp, err := client.GetTransaction(cmd.Context(), txHash)
	if err != nil {
		return fmt.Errorf("failed to fetch transaction: %w", err)
	}

	fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

	// TODO: Use d.Runner for simulation when ready
	// simReq := &simulator.SimulationRequest{
	//     EnvelopeXdr: resp.EnvelopeXdr,
	//     ResultMetaXdr: resp.ResultMetaXdr,
	// }
	// simResp, err := d.Runner.Run(simReq)

	return nil
}

var debugCmd = &cobra.Command{
	Use:   "debug <transaction-hash>",
	Short: "Debug a failed Soroban transaction",
	Long: `Fetch and simulate a Soroban transaction to debug failures and analyze execution.

This command retrieves the transaction envelope from the Stellar network, runs it
through the local simulator, and displays detailed execution traces including:
  - Transaction status and error messages
  - Contract events and diagnostic logs
  - Token flows (XLM and Soroban assets)
  - Execution metadata and state changes

The simulation results are stored in a session that can be saved for later analysis.

Local WASM Replay Mode:
  Use --wasm flag to test contracts locally without network data.`,
	Example: `  # Debug a transaction on mainnet
  erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab

  # Debug on testnet
  erst debug --network testnet abc123...def789

  # Debug and compare results between networks
  erst debug --network mainnet --compare-network testnet abc123...def789

  # Local WASM replay (no network required)
  erst debug --wasm ./contract.wasm --args "arg1" --args "arg2"`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Local WASM replay mode doesn't need transaction hash
		if wasmPath != "" {
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("transaction hash is required when not using --wasm flag")
		}

		if len(args[0]) != 64 {
			return fmt.Errorf("error: invalid transaction hash format (expected 64 hex characters, got %d)", len(args[0]))
		}

		// Validate network flag
		switch rpc.Network(networkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			// valid
		default:
			return errors.WrapInvalidNetwork(networkFlag)
		}

		// Validate compare network flag if present
		if compareNetworkFlag != "" {
			switch rpc.Network(compareNetworkFlag) {
			case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
				// valid
			default:
				return fmt.Errorf("invalid compare-network: %s. Must be one of: testnet, mainnet, futurenet", compareNetworkFlag)
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, cmdArgs []string) error {
		if verbose {
			logger.SetLevel(slog.LevelInfo)
		} else {
			logger.SetLevel(slog.LevelWarn)
		}

		// Local WASM replay mode
		if wasmPath != "" {
			return runLocalWasmReplay()
		}

		// Network transaction replay mode
		ctx := cmd.Context()
		txHash := cmdArgs[0]

		// Initialize OpenTelemetry if enabled
		if tracingEnabled {
			cleanup, err := telemetry.Init(ctx, telemetry.Config{
				Enabled:     true,
				ExporterURL: otlpExporterURL,
				ServiceName: "erst",
			})
			if err != nil {
				return fmt.Errorf("failed to initialize telemetry: %w", err)
			}
			defer cleanup()
		}

		// Start root span
		tracer := telemetry.GetTracer()
		ctx, span := tracer.Start(ctx, "debug_transaction")
		span.SetAttributes(
			attribute.String("transaction.hash", txHash),
			attribute.String("network", networkFlag),
		)
		defer span.End()

		client := rpc.NewClient(rpc.Network(networkFlag), rpcTokenFlag)
		horizonURL := ""
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag), rpcTokenFlag)
			horizonURL = rpcURLFlag
		} else {
			switch rpc.Network(networkFlag) {
			case rpc.Testnet:
				horizonURL = rpc.TestnetHorizonURL
			case rpc.Futurenet:
				horizonURL = rpc.FuturenetHorizonURL
			default:
				horizonURL = rpc.MainnetHorizonURL
			}
		}

		fmt.Printf("Fetching transaction: %s\n", txHash)
		resp, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			return fmt.Errorf(localization.Get("error.fetch_transaction"), err)
		}

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

		// Extract ledger keys for replay
		keys, err := extractLedgerKeys(resp.ResultMetaXdr)
		if err != nil {
			return fmt.Errorf("failed to extract ledger keys: %w", err)
		}

		runner, err := simulator.NewRunner("", tracingEnabled)
		if err != nil {
			return fmt.Errorf("failed to initialize simulator: %w", err)
		}

		// Determine timestamps to simulate
		timestamps := []int64{TimestampFlag}
		if WindowFlag > 0 && TimestampFlag > 0 {
			// Simulate 5 steps across the window
			step := WindowFlag / 4
			for i := 1; i <= 4; i++ {
				timestamps = append(timestamps, TimestampFlag+int64(i)*step)
			}
		}

		var lastSimResp *simulator.SimulationResponse

		for _, ts := range timestamps {
			if len(timestamps) > 1 {
				fmt.Printf("\n--- Simulating at Timestamp: %d ---\n", ts)
			}

			var simResp *simulator.SimulationResponse
			var ledgerEntries map[string]string

			if compareNetworkFlag == "" {
				// Single Network Run
				if snapshotFlag != "" {
					snap, err := snapshot.Load(snapshotFlag)
					if err != nil {
						return fmt.Errorf("failed to load snapshot: %w", err)
					}
					ledgerEntries = snap.ToMap()
				} else {
					ledgerEntries, err = client.GetLedgerEntries(ctx, keys)
					if err != nil {
						return fmt.Errorf("failed to fetch ledger entries: %w", err)
					}
				}

				fmt.Printf("Running simulation on %s...\n", networkFlag)
				simReq := &simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: ledgerEntries,
				}

				var err error
				simResp, err = runner.Run(simReq)
				if err != nil {
					return fmt.Errorf("simulation failed: %w", err)
				}
				printSimulationResult(networkFlag, simResp)
			} else {
				// Comparison Run
				var wg sync.WaitGroup
				var primaryResult, compareResult *simulator.SimulationResponse
				var primaryErr, compareErr error

				wg.Add(2)
				go func() {
					defer wg.Done()
					entries, err := client.GetLedgerEntries(ctx, keys)
					if err != nil {
						primaryErr = err
						return
					}
					primaryResult, primaryErr = runner.Run(&simulator.SimulationRequest{
						EnvelopeXdr:   resp.EnvelopeXdr,
						ResultMetaXdr: resp.ResultMetaXdr,
						LedgerEntries: entries,
						Timestamp:     ts,
					})
				}()

				go func() {
					defer wg.Done()
					compareClient := rpc.NewClient(rpc.Network(compareNetworkFlag), rpcTokenFlag)
					entries, err := compareClient.GetLedgerEntries(ctx, keys)
					if err != nil {
						compareErr = err
						return
					}
					compareResult, compareErr = runner.Run(&simulator.SimulationRequest{
						EnvelopeXdr:   resp.EnvelopeXdr,
						ResultMetaXdr: resp.ResultMetaXdr,
						LedgerEntries: entries,
						Timestamp:     ts,
					})
				}()

				wg.Wait()
				if primaryErr != nil {
					return fmt.Errorf("primary network error: %w", primaryErr)
				}
				if compareErr != nil {
					return fmt.Errorf("compare network error: %w", compareErr)
				}

				simResp = primaryResult // Use primary for further analysis
				printSimulationResult(networkFlag, primaryResult)
				printSimulationResult(compareNetworkFlag, compareResult)
				diffResults(primaryResult, compareResult, networkFlag, compareNetworkFlag)
			}
			lastSimResp = simResp
		}

		if lastSimResp == nil {
			return fmt.Errorf("no simulation results generated")
		}

		// Analysis: Security
		fmt.Printf("\n=== Security Analysis ===\n")
		secDetector := security.NewDetector()
		findings := secDetector.Analyze(resp.EnvelopeXdr, resp.ResultMetaXdr, lastSimResp.Events, lastSimResp.Logs)
		if len(findings) == 0 {
			fmt.Println("✓ No security issues detected")
		} else {
			for i, f := range findings {
				fmt.Printf("%d. [%s] %s: %s\n", i+1, f.Severity, f.Title, f.Description)
			}
		}

		// Analysis: Token Flows
		if report, err := tokenflow.BuildReport(resp.EnvelopeXdr, resp.ResultMetaXdr); err == nil && len(report.Agg) > 0 {
			fmt.Printf("\nToken Flow Summary:\n")
			for _, line := range report.SummaryLines() {
				fmt.Printf("  %s\n", line)
			}
			fmt.Printf("\nToken Flow Chart (Mermaid):\n")
			fmt.Println(report.MermaidFlowchart())
		}

		// Session Management
		sessionData := &session.SessionData{
			ID:            txHash[:8], // Simplified ID
			CreatedAt:     time.Now(),
			Network:       networkFlag,
			HorizonURL:    horizonURL,
			TxHash:        txHash,
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
		}
		SetCurrentSession(sessionData)
		fmt.Printf("\nSession ready. Use 'erst session save' to persist.\n")
		return nil
	},
}

func runLocalWasmReplay() error {
	fmt.Println("⚠️  WARNING: Using Mock State (not mainnet data)")
	fmt.Println()

	// Verify WASM file exists
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		return fmt.Errorf("WASM file not found: %s", wasmPath)
	}

	fmt.Println("🔧 Local WASM Replay Mode")
	fmt.Printf("WASM File: %s\n", wasmPath)
	fmt.Printf("Arguments: %v\n", args)
	fmt.Println()

	// Create simulator runner
	runner, err := simulator.NewRunner("", tracingEnabled)
	if err != nil {
		return fmt.Errorf("failed to initialize simulator: %w", err)
	}

	// Create simulation request with local WASM
	req := &simulator.SimulationRequest{
		EnvelopeXdr:   "",  // Empty for local replay
		ResultMetaXdr: "",  // Empty for local replay
		LedgerEntries: nil, // Mock state will be generated
		WasmPath:      &wasmPath,
		MockArgs:      &args,
	}

	// Run simulation
	fmt.Println("▶ Executing contract locally...")
	resp, err := runner.Run(req)
	if err != nil {
		fmt.Printf("✗ Execution failed: %v\n", err)
		return err
	}

	// Display results
	fmt.Println()
	fmt.Println("✓ Execution completed successfully")
	fmt.Println()

	if len(resp.Logs) > 0 {
		fmt.Println("📋 Logs:")
		for _, log := range resp.Logs {
			fmt.Printf("  %s\n", log)
		}
		fmt.Println()
	}

	if len(resp.Events) > 0 {
		fmt.Println("📡 Events:")
		for _, event := range resp.Events {
			fmt.Printf("  %s\n", event)
		}
		fmt.Println()
	}

	if verbose {
		fmt.Println("🔍 Full Response:")
		jsonBytes, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jsonBytes))
	}

	return nil
}

func extractLedgerKeys(metaXdr string) ([]string, error) {
	data, err := base64.StdEncoding.DecodeString(metaXdr)
	if err != nil {
		return nil, err
	}

	var meta xdr.TransactionResultMeta
	if err := xdr.SafeUnmarshal(data, &meta); err != nil {
		return nil, err
	}

	keysMap := make(map[string]struct{})
	addKey := func(k xdr.LedgerKey) {
		b, _ := k.MarshalBinary()
		keysMap[base64.StdEncoding.EncodeToString(b)] = struct{}{}
	}

	collectChanges := func(changes xdr.LedgerEntryChanges) {
		for _, c := range changes {
			switch c.Type {
			case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
				k, err := c.Created.LedgerKey()
				if err == nil {
					addKey(k)
				}
			case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
				k, err := c.Updated.LedgerKey()
				if err == nil {
					addKey(k)
				}
			case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
				if c.Removed != nil {
					addKey(*c.Removed)
				}
			case xdr.LedgerEntryChangeTypeLedgerEntryState:
				k, err := c.State.LedgerKey()
				if err == nil {
					addKey(k)
				}
			}
		}
	}

	// 1. Fee processing changes
	collectChanges(meta.FeeProcessing)

	// 2. Transaction apply processing changes
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
	return res, nil
}

func printSimulationResult(network string, res *simulator.SimulationResponse) {
	fmt.Printf("\n--- Result for %s ---\n", network)
	fmt.Printf("Status: %s\n", res.Status)
	if res.Error != "" {
		fmt.Printf("Error: %s\n", res.Error)
	}

	// Display budget usage if available
	if res.BudgetUsage != nil {
		fmt.Printf("\nResource Usage:\n")
		fmt.Printf("  CPU Instructions: %d\n", res.BudgetUsage.CPUInstructions)
		fmt.Printf("  Memory Bytes: %d\n", res.BudgetUsage.MemoryBytes)
		fmt.Printf("  Operations: %d\n", res.BudgetUsage.OperationsCount)
	}

	// Display diagnostic events with details
	if len(res.DiagnosticEvents) > 0 {
		fmt.Printf("\nDiagnostic Events: %d\n", len(res.DiagnosticEvents))
		for i, event := range res.DiagnosticEvents {
			if i < 10 { // Show first 10 events
				fmt.Printf("  [%d] Type: %s", i+1, event.EventType)
				if event.ContractID != nil {
					fmt.Printf(", Contract: %s", *event.ContractID)
				}
				fmt.Printf("\n")
				if len(event.Topics) > 0 {
					fmt.Printf("      Topics: %v\n", event.Topics)
				}
				if event.Data != "" && len(event.Data) < 100 {
					fmt.Printf("      Data: %s\n", event.Data)
				}
			}
		}
		if len(res.DiagnosticEvents) > 10 {
			fmt.Printf("  ... and %d more events\n", len(res.DiagnosticEvents)-10)
		}
	} else {
		fmt.Printf("\nEvents: %d\n", len(res.Events))
	}

	// Display logs
	if len(res.Logs) > 0 {
		fmt.Printf("\nLogs: %d\n", len(res.Logs))
		for i, log := range res.Logs {
			if i < 5 { // Show first 5 logs
				fmt.Printf("  - %s\n", log)
			}
		}
		if len(res.Logs) > 5 {
			fmt.Printf("  ... and %d more logs\n", len(res.Logs)-5)
		}
	}
	fmt.Printf("Events: %d, Logs: %d\n", len(res.Events), len(res.Logs))
}

func diffResults(res1, res2 *simulator.SimulationResponse, net1, net2 string) {
	if res1.Status != res2.Status {
		fmt.Printf("\n[DIFF] Status mismatch: %s vs %s\n", res1.Status, res2.Status)
	}

	// Compare diagnostic events if available
	if len(res1.DiagnosticEvents) > 0 && len(res2.DiagnosticEvents) > 0 {
		if len(res1.DiagnosticEvents) != len(res2.DiagnosticEvents) {
			fmt.Printf("[DIFF] Diagnostic events count mismatch: %d vs %d\n",
				len(res1.DiagnosticEvents), len(res2.DiagnosticEvents))
		}
	} else if len(res1.Events) != len(res2.Events) {
		fmt.Printf("[DIFF] Events count mismatch: %d vs %d\n", len(res1.Events), len(res2.Events))
	}

	// Compare budget usage if available
	if res1.BudgetUsage != nil && res2.BudgetUsage != nil {
		if res1.BudgetUsage.CPUInstructions != res2.BudgetUsage.CPUInstructions {
			fmt.Printf("[DIFF] CPU instructions: %d vs %d\n",
				res1.BudgetUsage.CPUInstructions, res2.BudgetUsage.CPUInstructions)
		}
		if res1.BudgetUsage.MemoryBytes != res2.BudgetUsage.MemoryBytes {
			fmt.Printf("[DIFF] Memory bytes: %d vs %d\n",
				res1.BudgetUsage.MemoryBytes, res2.BudgetUsage.MemoryBytes)
		}
	}
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", "mainnet", "Stellar network")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom RPC URL")
	debugCmd.Flags().BoolVar(&tracingEnabled, "tracing", false, "Enable tracing")
	debugCmd.Flags().StringVar(&otlpExporterURL, "otlp-url", "http://localhost:4318", "OTLP URL")
	debugCmd.Flags().BoolVar(&generateTrace, "generate-trace", false, "Generate trace file")
	debugCmd.Flags().StringVar(&traceOutputFile, "trace-output", "", "Trace output file")
	debugCmd.Flags().StringVar(&snapshotFlag, "snapshot", "", "Snapshot file")
	debugCmd.Flags().StringVar(&compareNetworkFlag, "compare-network", "", "Network to compare")
	debugCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	debugCmd.Flags().StringVar(&wasmPath, "wasm", "", "Path to local WASM file for local replay (no network required)")
	debugCmd.Flags().StringSliceVar(&args, "args", []string{}, "Mock arguments for local replay (JSON array of strings)")

	rootCmd.AddCommand(debugCmd)
}
