// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/db"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/localization"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
)

var (
	networkFlag     string
	rpcURLFlag      string
	tracingEnabled  bool
	otlpExporterURL string
	generateTrace   bool
	traceOutputFile string
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
	tracingEnabled     bool
	otlpExporterURL    string
	snapshotFlag       string
	compareNetworkFlag string
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
	
	return cmd
}

func (d *DebugCommand) runDebug(cmd *cobra.Command, args []string) error {
	txHash := args[0]

	var client *rpc.Client
	if rpcURLFlag != "" {
		client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
	} else {
		client = rpc.NewClient(rpc.Network(networkFlag))
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
	Short: localization.Get("cli.debug.short"),
	Long:  localization.Get("cli.debug.long"),
	Args:  cobra.ExactArgs(1),
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
			return fmt.Errorf("error: invalid transaction hash format (expected 64 hex characters, got %d)", len(args[0]))
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		txHash := args[0]

		client := rpc.NewClient(rpc.Network(networkFlag))
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
		}

		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			return fmt.Errorf(localization.Get("error.fetch_transaction"), err)
		}

		fmt.Printf(localization.Get("output.transaction_envelope")+"\n", len(resp.EnvelopeXdr))
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

		// Setup Primary Client
		var client *rpc.Client
		var horizonURL string
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
			horizonURL = rpcURLFlag
		} else {
			client = rpc.NewClient(rpc.Network(networkFlag))
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

		// Fetch transaction details
		resp, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		span.SetAttributes(
			attribute.Int("envelope.size_bytes", len(resp.EnvelopeXdr)),
		)

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

		// Generate trace if requested
		if generateTrace {
			fmt.Println("Generating execution trace...")
			err := generateExecutionTrace(ctx, txHash, resp)
			if err != nil {
				fmt.Printf("Warning: Failed to generate trace: %v\n", err)
			} else {
				filename := traceOutputFile
				if filename == "" {
					filename = fmt.Sprintf("trace_%s.json", txHash[:8])
				}
				fmt.Printf("Execution trace saved to: %s\n", filename)
			}
		}

		// Run simulation
		simRunner, err := simulator.NewRunner()
		if err != nil {
			return err
		}

		simReq := &simulator.SimulationRequest{
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
			Profile:       ProfileFlag,
		}

		simResp, err := simRunner.Run(simReq)
		if err != nil {
			return err
		}

		fmt.Println("Simulation successful!")
		if simResp.Flamegraph != "" {
			err := os.WriteFile("profile.svg", []byte(simResp.Flamegraph), 0644)
			if err != nil {
				return fmt.Errorf("failed to save flamegraph: %w", err)
			}
			fmt.Println("Flamegraph saved to profile.svg")
		// 3. Extract Ledger Keys from ResultMeta
		keys, err := extractLedgerKeys(resp.ResultMetaXdr)
		if err != nil {
			return fmt.Errorf("failed to extract ledger keys: %w", err)
		}
		logger.Logger.Info("Extracted ledger keys", "count", len(keys))

		// Load snapshot if provided
		var ledgerEntries map[string]string
		if snapshotFlag != "" {
			snap, err := snapshot.Load(snapshotFlag)
			if err != nil {
				return fmt.Errorf("failed to load snapshot: %w", err)
			}
			ledgerEntries = snap.ToMap()
			fmt.Printf("Loaded %d ledger entries from snapshot\n", len(ledgerEntries))
		}

		// Initialize Simulator Runner
		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("failed to initialize simulator runner: %w", err)
		}

		// Run Simulations
		if compareNetworkFlag == "" {
			// Single Run
			primaryEntries, err := client.GetLedgerEntries(ctx, keys)
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
			primaryResult, err := runner.Run(ctx, primaryReq)
			if err != nil {
				return fmt.Errorf("simulation failed on primary network: %w", err)
			}
			printSimulationResult(networkFlag, primaryResult)

			// Print token flow and security analysis for single network
			printTokenFlow(resp.EnvelopeXdr, resp.ResultMetaXdr)
			printSecurityAnalysis(resp.EnvelopeXdr, resp.ResultMetaXdr, primaryResult.Events, primaryResult.Logs)

		} else {
			// Parallel Execution
			var wg sync.WaitGroup
			var primaryResult, compareResult *simulator.SimulationResponse
			var primaryErr, compareErr error

			wg.Add(2)

			// Primary Network Routine
			go func() {
				defer wg.Done()

				primaryEntries, err := client.GetLedgerEntries(ctx, keys)
				if err != nil {
					primaryErr = fmt.Errorf("failed to fetch primary ledger entries: %w", err)
					return
				}
				fmt.Printf("Fetched %d ledger entries from %s\n", len(primaryEntries), networkFlag)

				fmt.Printf("Running simulation on %s...\n", networkFlag)
				primaryReq := &simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: primaryEntries,
				}
				primaryResult, primaryErr = runner.Run(ctx, primaryReq)
			}()

			// Compare Network Routine
			go func() {
				defer wg.Done()

				compareClient := rpc.NewClient(rpc.Network(compareNetworkFlag))

				compareEntries, err := compareClient.GetLedgerEntries(ctx, keys)
				if err != nil {
					compareErr = fmt.Errorf("failed to fetch ledger entries from %s: %w", compareNetworkFlag, err)
					return
				}
				fmt.Printf("Fetched %d ledger entries from %s\n", len(compareEntries), compareNetworkFlag)

				fmt.Printf("Running simulation on %s...\n", compareNetworkFlag)
				compareReq := &simulator.SimulationRequest{
					EnvelopeXdr:   resp.EnvelopeXdr,
					ResultMetaXdr: resp.ResultMetaXdr,
					LedgerEntries: compareEntries,
				}
				compareResult, compareErr = runner.Run(ctx, compareReq)
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

			// Print token flow and security analysis
			printTokenFlow(resp.EnvelopeXdr, resp.ResultMetaXdr)
			printSecurityAnalysis(resp.EnvelopeXdr, resp.ResultMetaXdr, primaryResult.Events, primaryResult.Logs)
		}

		// Create session data
		simReqJSON, err := json.Marshal(&simulator.SimulationRequest{
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
			LedgerEntries: ledgerEntries,
		})
		if err != nil {
			return fmt.Errorf("failed to serialize simulation data: %w", err)
		}

		simRespJSON, err := json.Marshal(&simulator.SimulationResponse{})
		if err != nil {
			return fmt.Errorf("failed to serialize simulation results: %w", err)
		}

		sessionData := &session.SessionData{
			ID:              session.GenerateID(txHash),
			CreatedAt:       time.Now(),
			LastAccessAt:    time.Now(),
			Status:          "active",
			Network:         networkFlag,
			HorizonURL:      horizonURL,
			TxHash:          txHash,
			EnvelopeXdr:     resp.EnvelopeXdr,
			ResultXdr:       resp.ResultXdr,
			ResultMetaXdr:   resp.ResultMetaXdr,
			SimRequestJSON:  string(simReqJSON),
			SimResponseJSON: string(simRespJSON),
			ErstVersion:     getErstVersion(),
			SchemaVersion:   session.SchemaVersion,
		}

		SetCurrentSession(sessionData)
		fmt.Printf("\nSession created: %s\n", sessionData.ID)
		fmt.Printf("Run 'erst session save' to persist this session.\n")

		return nil
	},
}

func extractLedgerKeys(metaXdr string) ([]string, error) {
	data, err := base64.StdEncoding.DecodeString(metaXdr)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	var meta xdr.TransactionResultMeta
	if err := xdr.SafeUnmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("xdr unmarshal failed: %w", err)
	}

	keysMap := make(map[string]struct{})

	collectChanges := func(l xdr.LedgerEntryChanges) {
		for _, change := range l {
			var key xdr.LedgerKey
			var err error
			switch change.Type {
			case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
				key, err = change.Created.LedgerKey()
			case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
				key, err = change.Updated.LedgerKey()
			case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
				if change.Removed != nil {
					key = *change.Removed
				} else {
					continue
				}
			case xdr.LedgerEntryChangeTypeLedgerEntryState:
				key, err = change.State.LedgerKey()
			}

			if err == nil {
				keyBytes, _ := key.MarshalBinary()
				keyB64 := base64.StdEncoding.EncodeToString(keyBytes)
				keysMap[keyB64] = struct{}{}
			}
		}
	}

	// 1. Fee Processing Changes
	collectChanges(meta.FeeProcessing)

	// 2. Transaction Application Changes
	switch meta.TxApplyProcessing.V {
	case 0:
		if ops, ok := meta.TxApplyProcessing.GetOperations(); ok {
			for _, op := range ops {
				collectChanges(op.Changes)
			}
		}
	case 1:
		if v1, ok := meta.TxApplyProcessing.GetV1(); ok {
			collectChanges(v1.TxChanges)
			for _, op := range v1.Operations {
				collectChanges(op.Changes)
			}
		}
	case 2:
		if v2, ok := meta.TxApplyProcessing.GetV2(); ok {
			collectChanges(v2.TxChangesBefore)
			collectChanges(v2.TxChangesAfter)
			for _, op := range v2.Operations {
				collectChanges(op.Changes)
			}
		}
	case 3:
		if v3, ok := meta.TxApplyProcessing.GetV3(); ok {
			collectChanges(v3.TxChangesBefore)
			collectChanges(v3.TxChangesAfter)
			for _, op := range v3.Operations {
				collectChanges(op.Changes)
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
		}
	}
}

func printTokenFlow(envelopeXdr, resultMetaXdr string) {
	fmt.Printf("\n=== Token Flow Summary ===\n")
	if report, err := tokenflow.BuildReport(envelopeXdr, resultMetaXdr); err != nil {
		fmt.Printf("(failed to parse: %v)\n", err)
	} else if len(report.Agg) == 0 {
		fmt.Printf("no transfers/mints detected\n")
	} else {
		for _, line := range report.SummaryLines() {
			fmt.Printf("  %s\n", line)
		}
		fmt.Printf("\nToken Flow Chart (Mermaid):\n")
		fmt.Println(report.MermaidFlowchart())
	}
}

func printSecurityAnalysis(envelopeXdr, resultMetaXdr string, events, logs []string) {
	fmt.Printf("\n=== Security Analysis ===\n")
	secDetector := security.NewDetector()
	findings := secDetector.Analyze(envelopeXdr, resultMetaXdr, events, logs)

	if len(findings) == 0 {
		fmt.Printf("No security issues detected\n")
		return
	}

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
		fmt.Printf("\nVERIFIED SECURITY RISKS: %d\n", verifiedCount)
	}
	if heuristicCount > 0 {
		fmt.Printf("HEURISTIC WARNINGS: %d\n", heuristicCount)
	}

	fmt.Printf("\nFindings:\n")
	for i, finding := range findings {
		icon := "HEURISTIC"
		if finding.Type == security.FindingVerifiedRisk {
			icon = "VERIFIED"
		}

		fmt.Printf("\n%d. [%s] %s - %s\n", i+1, icon, finding.Severity, finding.Title)
		fmt.Printf("   %s\n", finding.Description)
		if finding.Evidence != "" {
			fmt.Printf("   Evidence: %s\n", finding.Evidence)
		}
	}
}

// getErstVersion returns a version string for the current build
func getErstVersion() string {
	return "dev"
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), localization.Get("cli.debug.flag.network"))
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", localization.Get("cli.debug.flag.rpc_url"))
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	debugCmd.Flags().BoolVar(&tracingEnabled, "tracing", false, "Enable OpenTelemetry tracing")
	debugCmd.Flags().StringVar(&otlpExporterURL, "otlp-url", "http://localhost:4318", "OTLP exporter URL")
	debugCmd.Flags().BoolVar(&generateTrace, "generate-trace", false, "Generate execution trace file")
	debugCmd.Flags().StringVar(&traceOutputFile, "trace-output", "", "Output file for execution trace (default: trace_<txhash>.json)")
	debugCmd.Flags().StringVar(&snapshotFlag, "snapshot", "", "Load state from JSON snapshot file")
	debugCmd.Flags().StringVar(&compareNetworkFlag, "compare-network", "", "Network to compare against (testnet, mainnet, futurenet)")

	rootCmd.AddCommand(debugCmd)
}

// generateExecutionTrace creates a mock execution trace for demonstration
func generateExecutionTrace(ctx context.Context, txHash string, resp *rpc.TransactionResponse) error {
	// Create a mock simulation request
	simReq := &simulator.SimulationRequest{
		EnvelopeXdr:   resp.EnvelopeXdr,
		ResultMetaXdr: resp.ResultMetaXdr,
	}

	// Create simulator runner
	runner, err := simulator.NewRunner()
	if err != nil {
		return fmt.Errorf("failed to create simulator: %w", err)
	}

	// Run simulation with trace generation
	_, executionTrace, err := runner.RunWithTrace(ctx, simReq, txHash)
	if err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	// Save trace to file
	filename := traceOutputFile
	if filename == "" {
		filename = fmt.Sprintf("trace_%s.json", txHash[:8])
	}

	traceData, err := executionTrace.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize trace: %w", err)
	}

	err = os.WriteFile(filename, traceData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write trace file: %w", err)
	}

	return nil
}
