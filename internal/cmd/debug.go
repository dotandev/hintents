// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
)

var (
	networkFlag     string
	rpcURLFlag      string
	tracingEnabled  bool
	otlpExporterURL string
	"github.com/dotandev/hintents/internal/security"
	"github.com/dotandev/hintents/internal/session"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/dotandev/hintents/internal/tokenflow"
	"github.com/spf13/cobra"
	"github.com/stellar/go/xdr"
)

var (
	networkFlag  string
	rpcURLFlag   string
	snapshotFlag string
	networkFlag        string
	rpcURLFlag         string
	compareNetworkFlag string
)

var debugCmd = &cobra.Command{
	Use:   "debug <transaction-hash>",
	Short: "Debug a failed Soroban transaction",
	Long: `Fetch and simulate a Soroban transaction to debug failures and analyze execution.

This command retrieves the transaction envelope from the Stellar network, runs it
through the local simulator, and displays detailed execution traces including:
  • Transaction status and error messages
  • Contract events and diagnostic logs
  • Token flows (XLM and Soroban assets)
  • Execution metadata and state changes

The simulation results are stored in a session that can be saved for later analysis.`,
	Example: `  # Debug a transaction on mainnet
  erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab

  # Debug on testnet
  erst debug --network testnet abc123...def789

  # Use custom RPC endpoint
  erst debug --rpc-url https://custom-horizon.example.com abc123...def789

  # Debug and save the session
  erst debug abc123...def789 && erst session save`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args[0]) != 64 {
			return fmt.Errorf("Error: invalid transaction hash format (expected 64 hex characters, got %d)", len(args[0]))
		}
  erst debug --network testnet <tx-hash>
  erst debug --network mainnet --compare-network testnet <tx-hash>`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate network flag
		switch rpc.Network(networkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			// valid
		default:
			return fmt.Errorf("Error: %w", errors.WrapInvalidNetwork(networkFlag))
			return fmt.Errorf("invalid network: %s. Must be one of: testnet, mainnet, futurenet", networkFlag)
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
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		txHash := args[0]

		// Initialize OpenTelemetry if enabled
		var cleanup func()
		if tracingEnabled {
			var err error
			cleanup, err = telemetry.Init(ctx, telemetry.Config{
				Enabled:     true,
				ExporterURL: otlpExporterURL,
				ServiceName: "erst",
			})
			if err != nil {
				return fmt.Errorf("failed to initialize telemetry: %w", err)
			}
			defer cleanup()
		}

		// Start root span for transaction debugging
		tracer := telemetry.GetTracer()
		ctx, span := tracer.Start(ctx, "debug_transaction")
		span.SetAttributes(
			attribute.String("transaction.hash", txHash),
			attribute.String("network", networkFlag),
		)
		defer span.End()

		// 1. Setup Primary Client
		var client *rpc.Client
		var horizonURL string
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
			horizonURL = rpcURLFlag
		} else {
			client = rpc.NewClient(rpc.Network(networkFlag))
			// Get default Horizon URL for the network
			switch rpc.Network(networkFlag) {
			case rpc.Testnet:
				horizonURL = rpc.TestnetHorizonURL
			case rpc.Futurenet:
				horizonURL = rpc.FuturenetHorizonURL
			default:
				horizonURL = rpc.MainnetHorizonURL
			}
		}

		fmt.Printf("Debugging transaction: %s\n", txHash)
		fmt.Printf("Primary Network: %s\n", networkFlag)
		if compareNetworkFlag != "" {
			fmt.Printf("Comparing against Network: %s\n", compareNetworkFlag)
		}

		// Fetch transaction details with tracing
		ctx, fetchSpan := tracer.Start(ctx, "fetch_transaction")
		resp, err := client.GetTransaction(ctx, txHash)
		fetchSpan.End()

		// 2. Fetch transaction details from Primary Network
		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		span.SetAttributes(
			attribute.Int("envelope.size_bytes", len(resp.EnvelopeXdr)),
		)

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

		// 3. Extract Ledger Keys from ResultMeta
		keys, err := extractLedgerKeys(resp.ResultMetaXdr)
		if err != nil {
			return fmt.Errorf("failed to extract ledger keys: %w", err)
		}
		logger.Logger.Info("Extracted ledger keys", "count", len(keys))

		// 4. Initialize Simulator Runner
		// Fetch transaction details
		txResp, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			return fmt.Errorf("Error: failed to fetch transaction from network: %w", err)
		}

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(txResp.EnvelopeXdr))

		// Run simulation
		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("Error: failed to initialize simulator (ensure simulator binary is available): %w", err)
			return fmt.Errorf("failed to initialize simulator runner: %w", err)
		}

		// 5. Run Simulations
		if compareNetworkFlag == "" {
			// Single Run
			// Fetch Ledger Entries
			primaryEntries, err := client.GetLedgerEntries(cmd.Context(), keys)
			if err != nil {
				return fmt.Errorf("failed to fetch primary ledger entries: %w", err)
			}
			fmt.Printf("Fetched %d ledger entries from %s\n", len(primaryEntries), networkFlag)

			fmt.Printf("Running simulation on %s...\n", networkFlag)
			primaryReq := &simulator.SimulationRequest{
				EnvelopeXdr:   resp.EnvelopeXdr,
				ResultMetaXdr: resp.ResultMetaXdr,
				LedgerEntries: primaryEntries,
			}
			primaryResult, err := runner.Run(primaryReq)
			if err != nil {
				return fmt.Errorf("simulation failed on primary network: %w", err)
			}
			printSimulationResult(networkFlag, primaryResult)

		} else {
			// Parallel Execution
			var wg sync.WaitGroup
			var primaryResult, compareResult *simulator.SimulationResponse
			var primaryErr, compareErr error

			wg.Add(2)

			// Primary Network Routine
			go func() {
				defer wg.Done()
				
				// Fetch entries
				primaryEntries, err := client.GetLedgerEntries(cmd.Context(), keys)
				if err != nil {
					primaryErr = fmt.Errorf("failed to fetch primary ledger entries: %w", err)
					return
				}
				fmt.Printf("Fetched %d ledger entries from %s\n", len(primaryEntries), networkFlag)

				// Run Simulation
				fmt.Printf("Running simulation on %s...\n", networkFlag)
				primaryReq := &simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: primaryEntries,
				}
				primaryResult, primaryErr = runner.Run(primaryReq)
			}()

			// Compare Network Routine
			go func() {
				defer wg.Done()
				
				compareClient := rpc.NewClient(rpc.Network(compareNetworkFlag))
				
				// Fetch entries
				compareEntries, err := compareClient.GetLedgerEntries(cmd.Context(), keys)
				if err != nil {
					compareErr = fmt.Errorf("failed to fetch ledger entries from %s: %w", compareNetworkFlag, err)
					return
				}
				fmt.Printf("Fetched %d ledger entries from %s\n", len(compareEntries), compareNetworkFlag)

				// Run Simulation
				fmt.Printf("Running simulation on %s...\n", compareNetworkFlag)
				compareReq := &simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: compareEntries,
				}
				compareResult, compareErr = runner.Run(compareReq)
			}()

			wg.Wait()

			if primaryErr != nil {
				return fmt.Errorf("error on primary network: %w", primaryErr)
			}
			if compareErr != nil {
				return fmt.Errorf("error on compare network: %w", compareErr)
			}

			// Print and Diff
			printSimulationResult(networkFlag, primaryResult)
			printSimulationResult(compareNetworkFlag, compareResult)
			diffResults(primaryResult, compareResult, networkFlag, compareNetworkFlag)
		}

		var ledgerEntries map[string]string
		if snapshotFlag != "" {
			snap, err := snapshot.Load(snapshotFlag)
			if err != nil {
				return fmt.Errorf("failed to load snapshot: %w", err)
			}
			ledgerEntries = snap.ToMap()
			fmt.Printf("Loaded %d ledger entries from snapshot\n", len(ledgerEntries))
		}

		return nil
	},
}

func extractLedgerKeys(metaXdr string) ([]string, error) {
	// Decode Base64
	data, err := base64.StdEncoding.DecodeString(metaXdr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	// Unmarshal XDR
	var meta xdr.TransactionResultMeta
	if err := xdr.SafeUnmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("xdr unmarshal failed: %w", err)
	}

	keysMap := make(map[string]struct{})

	// Helper to add key
	addKey := func(k xdr.LedgerKey) error {
		keyBytes, err := k.MarshalBinary()
		// Build simulation request
		simReq := &simulator.SimulationRequest{
			EnvelopeXdr:   txResp.EnvelopeXdr,
			ResultMetaXdr: txResp.ResultMetaXdr,
			LedgerEntries: ledgerEntries,
		}

		fmt.Printf("Running simulation...\n")
		simResp, err := runner.Run(simReq)
		if err != nil {
			return fmt.Errorf("Error: simulation failed: %w", err)
			return err
		}
		keyB64 := base64.StdEncoding.EncodeToString(keyBytes)
		keysMap[keyB64] = struct{}{}
		return nil
	}

	// Iterate over changes
	var changes []xdr.LedgerEntryChange

	// Helper to collect changes from different versions
	collectChanges := func(l xdr.LedgerEntryChanges) {
		changes = append(changes, l...)
	}

	switch meta.V {
	case 0:
		collectChanges(meta.Operations)
	case 1:
		collectChanges(meta.V1.TxApplyProcessing.FeeProcessing)
		collectChanges(meta.V1.TxApplyProcessing.TxApplyProcessing)
	case 2:
		collectChanges(meta.V2.TxApplyProcessing.FeeProcessing)
		collectChanges(meta.V2.TxApplyProcessing.TxApplyProcessing)
	case 3:
		collectChanges(meta.V3.TxApplyProcessing.FeeProcessing)
		collectChanges(meta.V3.TxApplyProcessing.TxApplyProcessing)
	}

	for _, change := range changes {
		switch change.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			if err := addKey(change.Created.LedgerKey()); err != nil {
				return nil, err
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			if err := addKey(change.Updated.LedgerKey()); err != nil {
				return nil, err
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			if err := addKey(change.Removed); err != nil {
				return nil, err
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			if err := addKey(change.State.LedgerKey()); err != nil {
				return nil, err
		// Display simulation results
		fmt.Printf("\nSimulation Results:\n")
		fmt.Printf("  Status: %s\n", simResp.Status)
		if simResp.Error != "" {
			fmt.Printf("  Error: %s\n", simResp.Error)
		}
		if len(simResp.Events) > 0 {
			fmt.Printf("  Events: %d\n", len(simResp.Events))
			for i, event := range simResp.Events {
				if i < 5 { // Show first 5 events
					fmt.Printf("    - %s\n", event)
				}
			}
			if len(simResp.Events) > 5 {
				fmt.Printf("    ... and %d more\n", len(simResp.Events)-5)
			}
		}
		if len(simResp.Logs) > 0 {
			fmt.Printf("  Logs: %d\n", len(simResp.Logs))
			for i, log := range simResp.Logs {
				if i < 5 { // Show first 5 logs
					fmt.Printf("    - %s\n", log)
				}
			}
			if len(simResp.Logs) > 5 {
				fmt.Printf("    ... and %d more\n", len(simResp.Logs)-5)
			}
		}
	}

	result := make([]string, 0, len(keysMap))
	for k := range keysMap {
		result = append(result, k)
	}
	return result, nil
}

func printSimulationResult(network string, res *simulator.SimulationResponse) {
	fmt.Printf("\n--- Result for %s ---\n", network)
	fmt.Printf("Status: %s\n", res.Status)
	if res.Error != "" {
		fmt.Printf("Error: %s\n", res.Error)
	}
	fmt.Printf("Events: %d\n", len(res.Events))
	for i, ev := range res.Events {
		fmt.Printf("  [%d] %s\n", i, ev)
	}
}

func diffResults(res1, res2 *simulator.SimulationResponse, net1, net2 string) {
	fmt.Printf("\n=== Comparison: %s vs %s ===\n", net1, net2)
	
	if res1.Status != res2.Status {
		fmt.Printf("Status Mismatch: %s (%s) vs %s (%s)\n", res1.Status, net1, res2.Status, net2)
	} else {
		fmt.Printf("Status Match: %s\n", res1.Status)
	}

	// Compare Events
	fmt.Println("\nEvent Diff:")
	maxEvents := len(res1.Events)
	if len(res2.Events) > maxEvents {
		maxEvents = len(res2.Events)
	}

	for i := 0; i < maxEvents; i++ {
		var ev1, ev2 string
		if i < len(res1.Events) {
			ev1 = res1.Events[i]
		} else {
			ev1 = "<missing>"
		}
		
		if i < len(res2.Events) {
			ev2 = res2.Events[i]
		} else {
			ev2 = "<missing>"
		}

		if ev1 != ev2 {
			fmt.Printf("  [%d] MISMATCH:\n", i)
			fmt.Printf("    %s: %s\n", net1, ev1)
			fmt.Printf("    %s: %s\n", net2, ev2)
		} else {
			// Optional: Print matches if verbose
		}
	}
		// Serialize simulation request/response for session storage
		simReqJSON, err := json.Marshal(simReq)
		if err != nil {
			return fmt.Errorf("Error: failed to serialize simulation data: %w", err)
		}
		simRespJSON, err := json.Marshal(simResp)
		if err != nil {
			return fmt.Errorf("Error: failed to serialize simulation results: %w", err)
		}

		// Create session data
		sessionData := &session.SessionData{
			ID:              session.GenerateID(txHash),
			CreatedAt:       time.Now(),
			LastAccessAt:    time.Now(),
			Status:          "active",
			Network:         networkFlag,
			HorizonURL:      horizonURL,
			TxHash:          txHash,
			EnvelopeXdr:     txResp.EnvelopeXdr,
			ResultXdr:       txResp.ResultXdr,
			ResultMetaXdr:   txResp.ResultMetaXdr,
			SimRequestJSON:  string(simReqJSON),
			SimResponseJSON: string(simRespJSON),
			ErstVersion:     getErstVersion(),
			SchemaVersion:   session.SchemaVersion,
		}

		// Token flow summary (native XLM + Soroban SAC via diagnostic events in ResultMetaXdr)
		if report, err := tokenflow.BuildReport(txResp.EnvelopeXdr, txResp.ResultMetaXdr); err != nil {
			fmt.Printf("\nToken Flow Summary: (failed to parse: %v)\n", err)
		} else if len(report.Agg) == 0 {
			fmt.Printf("\nToken Flow Summary: no transfers/mints detected\n")
		} else {
			fmt.Printf("\nToken Flow Summary:\n")
			for _, line := range report.SummaryLines() {
				fmt.Printf("  %s\n", line)
			}
			fmt.Printf("\nToken Flow Chart (Mermaid):\n")
			fmt.Println(report.MermaidFlowchart())
		}

		// Security vulnerability analysis
		fmt.Printf("\n=== Security Analysis ===\n")
		secDetector := security.NewDetector()
		findings := secDetector.Analyze(txResp.EnvelopeXdr, txResp.ResultMetaXdr, simResp.Events, simResp.Logs)

		if len(findings) == 0 {
			fmt.Printf("✓ No security issues detected\n")
		} else {
			verifiedCount := 0
			heuristicCount := 0

			for _, finding := range findings {
				if finding.Type == security.FindingVerifiedRisk {
					verifiedCount++
				} else {
					heuristicCount++
				}
			}

			if verifiedCount > 0 {
				fmt.Printf("\n⚠️  VERIFIED SECURITY RISKS: %d\n", verifiedCount)
			}
			if heuristicCount > 0 {
				fmt.Printf("⚡ HEURISTIC WARNINGS: %d\n", heuristicCount)
			}

			fmt.Printf("\nFindings:\n")
			for i, finding := range findings {
				icon := "⚡"
				if finding.Type == security.FindingVerifiedRisk {
					icon = "⚠️"
				}

				fmt.Printf("\n%d. %s [%s] %s - %s\n", i+1, icon, finding.Type, finding.Severity, finding.Title)
				fmt.Printf("   %s\n", finding.Description)
				if finding.Evidence != "" {
					fmt.Printf("   Evidence: %s\n", finding.Evidence)
				}
			}
		}

		// Store as current session for potential saving
		SetCurrentSession(sessionData)

		fmt.Printf("\nSession created: %s\n", sessionData.ID)
		fmt.Printf("Run 'erst session save' to persist this session.\n")

		return nil
	},
}

// getErstVersion returns a version string for the current build
func getErstVersion() string {
	// In a real build, this would come from build flags or version.go
	// For now, return a placeholder
	return "dev"
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	debugCmd.Flags().BoolVar(&tracingEnabled, "tracing", false, "Enable OpenTelemetry tracing")
	debugCmd.Flags().StringVar(&otlpExporterURL, "otlp-url", "http://localhost:4318", "OTLP exporter URL")
	debugCmd.Flags().StringVar(&snapshotFlag, "snapshot", "", "Load state from JSON snapshot file")
	debugCmd.Flags().StringVar(&compareNetworkFlag, "compare-network", "", "Network to compare against (testnet, mainnet, futurenet)")

	rootCmd.AddCommand(debugCmd)
}
