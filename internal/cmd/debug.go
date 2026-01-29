// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/localization"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/security"
	"github.com/dotandev/hintents/internal/session"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/dotandev/hintents/internal/tokenflow"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stellar/go/xdr"
	"go.opentelemetry.io/otel/attribute"
)

var (
	networkFlag        string
	rpcURLFlag         string
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

		client := rpc.NewClient(rpc.Network(networkFlag))
		horizonURL := ""
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
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

		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("failed to initialize simulator: %w", err)
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
				})
			}()

			go func() {
				defer wg.Done()
				compareClient := rpc.NewClient(rpc.Network(compareNetworkFlag))
				entries, err := compareClient.GetLedgerEntries(ctx, keys)
				if err != nil {
					compareErr = err
					return
				}
				compareResult, compareErr = runner.Run(&simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: entries,
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

		// Analysis: Security
		fmt.Printf("\n=== Security Analysis ===\n")
		secDetector := security.NewDetector()
		findings := secDetector.Analyze(resp.EnvelopeXdr, resp.ResultMetaXdr, simResp.Events, simResp.Logs)
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
	color.Yellow("⚠️  WARNING: Using Mock State (not mainnet data)")
	fmt.Println()

	// Verify WASM file exists
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		return fmt.Errorf("WASM file not found: %s", wasmPath)
	}

	color.Cyan("🔧 Local WASM Replay Mode")
	fmt.Printf("WASM File: %s\n", wasmPath)
	fmt.Printf("Arguments: %v\n", args)
	fmt.Println()

	// Create simulator runner
	runner, err := simulator.NewRunner()
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
	color.Green("▶ Executing contract locally...")
	resp, err := runner.Run(req)
	if err != nil {
		color.Red("✗ Execution failed: %v", err)
		return err
	}

	// Display results
	fmt.Println()
	color.Green("✓ Execution completed successfully")
	fmt.Println()

	if len(resp.Logs) > 0 {
		color.Cyan("📋 Logs:")
		for _, log := range resp.Logs {
			fmt.Printf("  %s\n", log)
		}
		fmt.Println()
	}

	if len(resp.Events) > 0 {
		color.Cyan("📡 Events:")
		for _, event := range resp.Events {
			fmt.Printf("  %s\n", event)
		}
		fmt.Println()
	}

	if verbose {
		color.Cyan("🔍 Full Response:")
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
	fmt.Printf("Events: %d, Logs: %d\n", len(res.Events), len(res.Logs))
}

func diffResults(res1, res2 *simulator.SimulationResponse, net1, net2 string) {
	if res1.Status != res2.Status {
		fmt.Printf("\n[DIFF] Status mismatch: %s vs %s\n", res1.Status, res2.Status)
	}
	if len(res1.Events) != len(res2.Events) {
		fmt.Printf("[DIFF] Events count mismatch: %d vs %d\n", len(res1.Events), len(res2.Events))
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