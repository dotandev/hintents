// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/security"
	"github.com/dotandev/hintents/internal/session"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/dotandev/hintents/internal/snapshot"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/dotandev/hintents/internal/tokenflow"
	"github.com/dotandev/hintents/internal/visualizer"
	"github.com/dotandev/hintents/internal/watch"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/xdr"
	"go.opentelemetry.io/otel/attribute"
)

var (
	networkFlag         string
	rpcURLFlag          string
	rpcTokenFlag        string
	tracingEnabled      bool
	otlpExporterURL     string
	generateTrace       bool
	traceOutputFile     string
	snapshotFlag        string
	compareNetworkFlag  string
	verbose             bool
	wasmPath            string
	args                []string
	noCacheFlag         bool
	demoMode            bool
	watchFlag           bool
	watchTimeoutFlag    int
	protocolVersionFlag uint32
	themeFlag           string
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
				return errors.WrapInvalidNetwork(networkFlag)
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

	token := rpcTokenFlag
	if token == "" {
		token = os.Getenv("ERST_RPC_TOKEN")
	}
	if token == "" {
		cfg, err := config.LoadConfig()
		if err == nil && cfg.RPCToken != "" {
			token = cfg.RPCToken
		}
	}

	opts := []rpc.ClientOption{
		rpc.WithNetwork(rpc.Network(networkFlag)),
		rpc.WithToken(token),
	}
	if rpcURLFlag != "" {
		opts = append(opts, rpc.WithHorizonURL(rpcURLFlag))
	}

	client, err := rpc.NewClient(opts...)
	if err != nil {
		return errors.WrapValidationError(fmt.Sprintf("failed to create client: %v", err))
	}

	fmt.Printf("Debugging transaction: %s\n", txHash)
	fmt.Printf("Network: %s\n", networkFlag)
	if rpcURLFlag != "" {
		fmt.Printf("RPC URL: %s\n", rpcURLFlag)
	}

	// Fetch transaction details
	resp, err := client.GetTransaction(cmd.Context(), txHash)
	if err != nil {
		return errors.WrapRPCConnectionFailed(err)
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

  # Debug and save the session
  erst debug abc123...def789 && erst session save

  # Compare execution across networks
  erst debug --network testnet --compare-network mainnet <tx-hash>

  # Local WASM replay (no network required)
  erst debug --wasm ./contract.wasm --args "arg1" --args "arg2"

  # Demo mode (test color output, no network required)
  erst debug --demo`,
	Args: cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Demo mode or local WASM replay don't need transaction hash
		if demoMode || wasmPath != "" {
			return nil
		}

		if len(args) == 0 {
			return errors.WrapValidationError("transaction hash is required when not using --wasm or --demo flag")
		}

		if err := rpc.ValidateTransactionHash(args[0]); err != nil {
			return errors.WrapValidationError(fmt.Sprintf("invalid transaction hash format: %v", err))
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
				return errors.WrapInvalidNetwork(compareNetworkFlag)
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

		// Apply theme if specified, otherwise auto-detect
		if themeFlag != "" {
			visualizer.SetTheme(visualizer.Theme(themeFlag))
		} else {
			visualizer.SetTheme(visualizer.DetectTheme())
		}

		// Demo mode: print sample output for testing color detection (no network)
		if demoMode {
			return runDemoMode(cmdArgs)
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
				return errors.WrapValidationError(fmt.Sprintf("failed to initialize telemetry: %v", err))
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

		var horizonURL string
		token := rpcTokenFlag
		if token == "" {
			token = os.Getenv("ERST_RPC_TOKEN")
		}
		if token == "" {
			if cfg, err := config.Load(); err == nil && cfg.RPCToken != "" {
				token = cfg.RPCToken
			}
		}

		opts := []rpc.ClientOption{
			rpc.WithNetwork(rpc.Network(networkFlag)),
			rpc.WithToken(token),
		}

		if rpcURLFlag != "" {
			urls := strings.Split(rpcURLFlag, ",")
			for i := range urls {
				urls[i] = strings.TrimSpace(urls[i])
			}
			opts = append(opts, rpc.WithAltURLs(urls))
			horizonURL = urls[0]
		} else {
			cfg, err := config.Load()
			if err == nil {
				if len(cfg.RpcUrls) > 0 {
					opts = append(opts, rpc.WithAltURLs(cfg.RpcUrls))
					horizonURL = cfg.RpcUrls[0]
				} else if cfg.RpcUrl != "" {
					opts = append(opts, rpc.WithHorizonURL(cfg.RpcUrl))
					horizonURL = cfg.RpcUrl
				}
			}
		}

		client, err := rpc.NewClient(opts...)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to create client: %v", err))
		}

		if horizonURL == "" {
			// Extract horizon URL from valid client if not explicitly set
			horizonURL = client.HorizonURL
		}

		if noCacheFlag {
			client.CacheEnabled = false
			fmt.Println("🚫 Cache disabled by --no-cache flag")
		}

		fmt.Printf("Debugging transaction: %s\n", txHash)
		fmt.Printf("Primary Network: %s\n", networkFlag)
		if compareNetworkFlag != "" {
			fmt.Printf("Comparing against Network: %s\n", compareNetworkFlag)
		}

		// Fetch transaction details
		if watchFlag {
			spinner := watch.NewSpinner()
			poller := watch.NewPoller(watch.PollerConfig{
				InitialInterval: 1 * time.Second,
				MaxInterval:     10 * time.Second,
				TimeoutDuration: time.Duration(watchTimeoutFlag) * time.Second,
			})

			spinner.Start("Waiting for transaction to appear on-chain...")

			result, err := poller.Poll(ctx, func(pollCtx context.Context) (interface{}, error) {
				_, pollErr := client.GetTransaction(pollCtx, txHash)
				if pollErr != nil {
					return nil, pollErr
				}
				return true, nil
			}, nil)

			if err != nil {
				spinner.StopWithError("Failed to poll for transaction")
				return errors.WrapSimulationLogicError(fmt.Sprintf("watch mode error: %v", err))
			}

			if !result.Found {
				spinner.StopWithError("Transaction not found within timeout")
				return errors.WrapTransactionNotFound(fmt.Errorf("not found after %d seconds", watchTimeoutFlag))
			}

			spinner.StopWithMessage("Transaction found! Starting debug...")
		}

		fmt.Printf("Fetching transaction: %s\n", txHash)
		resp, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			return errors.WrapRPCConnectionFailed(err)
		}

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

		// Extract ledger keys for replay
		keys, err := extractLedgerKeys(resp.ResultMetaXdr)
		if err != nil {
			return errors.WrapUnmarshalFailed(err, "result meta")
		}

		// Initialize Simulator Runner
		runner, err := simulator.NewRunner("", tracingEnabled)
		if err != nil {
			return errors.WrapSimulatorNotFound(err.Error())
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
						return errors.WrapValidationError(fmt.Sprintf("failed to load snapshot: %v", err))
					}
					ledgerEntries = snap.ToMap()
					fmt.Printf("Loaded %d ledger entries from snapshot\n", len(ledgerEntries))
				} else {
					// Try to extract from metadata first, fall back to fetching
					ledgerEntries, err = rpc.ExtractLedgerEntriesFromMeta(resp.ResultMetaXdr)
					if err != nil {
						logger.Logger.Warn("Failed to extract ledger entries from metadata, fetching from network", "error", err)
						ledgerEntries, err = client.GetLedgerEntries(ctx, keys)
						if err != nil {
							return errors.WrapRPCConnectionFailed(err)
						}
					} else {
						logger.Logger.Info("Extracted ledger entries for simulation", "count", len(ledgerEntries))
					}
				}

				fmt.Printf("Running simulation on %s...\n", networkFlag)
				simReq := &simulator.SimulationRequest{
					EnvelopeXdr:     resp.EnvelopeXdr,
					ResultMetaXdr:   resp.ResultMetaXdr,
					LedgerEntries:   ledgerEntries,
					Timestamp:       ts,
					ProtocolVersion: nil,
				}

				// Apply protocol version override if specified
				if protocolVersionFlag > 0 {
					if err := simulator.Validate(protocolVersionFlag); err != nil {
						return fmt.Errorf("invalid protocol version %d: %w", protocolVersionFlag, err)
					}
					simReq.ProtocolVersion = &protocolVersionFlag
					fmt.Printf("Using protocol version override: %d\n", protocolVersionFlag)
				}

				simResp, err = runner.Run(simReq)
				if err != nil {
					return errors.WrapSimulationFailed(err, "")
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
					var entries map[string]string
					var extractErr error
					entries, extractErr = rpc.ExtractLedgerEntriesFromMeta(resp.ResultMetaXdr)
					if extractErr != nil {
						entries, extractErr = client.GetLedgerEntries(ctx, keys)
						if extractErr != nil {
							primaryErr = extractErr
							return
						}
					}
					simReq := &simulator.SimulationRequest{
						EnvelopeXdr:     resp.EnvelopeXdr,
						ResultMetaXdr:   resp.ResultMetaXdr,
						LedgerEntries:   entries,
						Timestamp:       ts,
						ProtocolVersion: nil,
					}
					if protocolVersionFlag > 0 {
						if err := simulator.Validate(protocolVersionFlag); err != nil {
							primaryErr = fmt.Errorf("invalid protocol version %d: %w", protocolVersionFlag, err)
							return
						}
						simReq.ProtocolVersion = &protocolVersionFlag
					}
					primaryResult, primaryErr = runner.Run(simReq)
				}()

				go func() {
					defer wg.Done()
					compareOpts := []rpc.ClientOption{
						rpc.WithNetwork(rpc.Network(compareNetworkFlag)),
						rpc.WithToken(rpcTokenFlag),
					}
					compareClient, clientErr := rpc.NewClient(compareOpts...)
					if clientErr != nil {
						compareErr = errors.WrapValidationError(fmt.Sprintf("failed to create compare client: %v", clientErr))
						return
					}
					if noCacheFlag {
						compareClient.CacheEnabled = false
					}

					compareResp, txErr := compareClient.GetTransaction(ctx, txHash)
					if txErr != nil {
						compareErr = errors.WrapRPCConnectionFailed(txErr)
						return
					}

					entries, extractErr := rpc.ExtractLedgerEntriesFromMeta(compareResp.ResultMetaXdr)
					if extractErr != nil {
						entries, extractErr = compareClient.GetLedgerEntries(ctx, keys)
						if extractErr != nil {
							compareErr = extractErr
							return
						}
					}

					simReq := &simulator.SimulationRequest{
						EnvelopeXdr:     resp.EnvelopeXdr,
						ResultMetaXdr:   compareResp.ResultMetaXdr,
						LedgerEntries:   entries,
						Timestamp:       ts,
						ProtocolVersion: nil,
					}
					if protocolVersionFlag > 0 {
						if err := simulator.Validate(protocolVersionFlag); err != nil {
							compareErr = fmt.Errorf("invalid protocol version %d: %w", protocolVersionFlag, err)
							return
						}
						simReq.ProtocolVersion = &protocolVersionFlag
					}
					compareResult, compareErr = runner.Run(simReq)
				}()

				wg.Wait()
				if primaryErr != nil {
					return errors.WrapRPCConnectionFailed(primaryErr)
				}
				if compareErr != nil {
					return errors.WrapRPCConnectionFailed(compareErr)
				}

				simResp = primaryResult // Use primary for further analysis
				printSimulationResult(networkFlag, primaryResult)
				printSimulationResult(compareNetworkFlag, compareResult)
				diffResults(primaryResult, compareResult, networkFlag, compareNetworkFlag)
			}
			lastSimResp = simResp
		}

		if lastSimResp == nil {
			return errors.WrapSimulationLogicError("no simulation results generated")
		}

		// Analysis: Security
		fmt.Printf("\n=== Security Analysis ===\n")
		secDetector := security.NewDetector()
		findings := secDetector.Analyze(resp.EnvelopeXdr, resp.ResultMetaXdr, lastSimResp.Events, lastSimResp.Logs)
		if len(findings) == 0 {
			fmt.Printf("%s No security issues detected\n", visualizer.Success())
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
				fmt.Printf("\n[!]  VERIFIED SECURITY RISKS: %d\n", verifiedCount)
			}
			if heuristicCount > 0 {
				fmt.Printf("* HEURISTIC WARNINGS: %d\n", heuristicCount)
			}

			fmt.Printf("\nFindings:\n")
			for i, finding := range findings {
				icon := "*"
				if finding.Type == security.FindingVerifiedRisk {
					icon = "[!]"
				}
				fmt.Printf("%d. %s [%s] %s - %s\n", i+1, icon, finding.Type, finding.Severity, finding.Title)
				fmt.Printf("   %s\n", finding.Description)
				if finding.Evidence != "" {
					fmt.Printf("   Evidence: %s\n", finding.Evidence)
				}
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
		simReq := &simulator.SimulationRequest{
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
		}
		simReqJSON, err := json.Marshal(simReq)
		if err != nil {
			fmt.Printf("Warning: failed to serialize simulation data: %v\n", err)
		}
		simRespJSON, err := json.Marshal(lastSimResp)
		if err != nil {
			fmt.Printf("Warning: failed to serialize simulation results: %v\n", err)
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
			ErstVersion:     Version,
			SchemaVersion:   session.SchemaVersion,
		}
		SetCurrentSession(sessionData)
		fmt.Printf("\nSession created: %s\n", sessionData.ID)
		fmt.Printf("Run 'erst session save' to persist this session.\n")
		return nil
	},
}

// runDemoMode prints sample output without network/WASM - for testing color detection.
func runDemoMode(cmdArgs []string) error {
	txHash := "5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	if len(cmdArgs) > 0 && len(cmdArgs[0]) == 64 {
		txHash = cmdArgs[0]
	}

	fmt.Printf("Fetching transaction: %s\n", txHash)
	fmt.Printf("Transaction fetched successfully. Envelope size: 256 bytes\n")
	fmt.Printf("\n--- Result for %s ---\n", networkFlag)
	fmt.Printf("Status: success\n")
	fmt.Printf("\nResource Usage:\n")
	fmt.Printf("  CPU Instructions: 12345\n")
	fmt.Printf("  Memory Bytes: 1024\n")
	fmt.Printf("  Operations: 5\n")
	fmt.Printf("\nEvents: 2, Logs: 3\n")
	fmt.Printf("\n=== Security Analysis ===\n")
	fmt.Printf("%s No security issues detected\n", visualizer.Success())
	fmt.Printf("\nToken Flow Summary:\n")
	fmt.Printf("  %s XLM transferred\n", visualizer.Symbol("arrow_r"))
	fmt.Printf("\nSession ready. Use 'erst session save' to persist.\n")
	return nil
}

func runLocalWasmReplay() error {
	fmt.Printf("%s  WARNING: Using Mock State (not mainnet data)\n", visualizer.Warning())
	fmt.Println()

	// Verify WASM file exists
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		return errors.WrapValidationError(fmt.Sprintf("WASM file not found: %s", wasmPath))
	}

	fmt.Printf("%s Local WASM Replay Mode\n", visualizer.Symbol("wrench"))
	fmt.Printf("WASM File: %s\n", wasmPath)
	fmt.Printf("Arguments: %v\n", args)
	fmt.Println()

	// Create simulator runner
	runner, err := simulator.NewRunner("", tracingEnabled)
	if err != nil {
		return errors.WrapSimulatorNotFound(err.Error())
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
	fmt.Printf("%s Executing contract locally...\n", visualizer.Symbol("play"))
	resp, err := runner.Run(req)
	if err != nil {
		fmt.Printf("%s Execution failed: %v\n", visualizer.Error(), err)
		return err
	}

	// Display results
	fmt.Println()
	fmt.Printf("%s Execution completed successfully\n", visualizer.Success())
	fmt.Println()

	if len(resp.Logs) > 0 {
		fmt.Printf("%s Logs:\n", visualizer.Symbol("logs"))
		for _, log := range resp.Logs {
			fmt.Printf("  %s\n", log)
		}
		fmt.Println()
	}

	if len(resp.Events) > 0 {
		fmt.Printf("%s Events:\n", visualizer.Symbol("events"))
		for _, event := range resp.Events {
			fmt.Printf("  %s\n", event)
		}
		fmt.Println()
	}

	if verbose {
		fmt.Printf("%s Full Response:\n", visualizer.Symbol("magnify"))
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

		// CPU usage with percentage and warning indicator
		cpuIndicator := ""
		if res.BudgetUsage.CPUUsagePercent >= 95.0 {
			cpuIndicator = " [!]  CRITICAL"
		} else if res.BudgetUsage.CPUUsagePercent >= 80.0 {
			cpuIndicator = " [!]  WARNING"
		}
		fmt.Printf("  CPU Instructions: %d / %d (%.2f%%)%s\n",
			res.BudgetUsage.CPUInstructions,
			res.BudgetUsage.CPULimit,
			res.BudgetUsage.CPUUsagePercent,
			cpuIndicator)

		// Memory usage with percentage and warning indicator
		memIndicator := ""
		if res.BudgetUsage.MemoryUsagePercent >= 95.0 {
			memIndicator = " [!]  CRITICAL"
		} else if res.BudgetUsage.MemoryUsagePercent >= 80.0 {
			memIndicator = " [!]  WARNING"
		}
		fmt.Printf("  Memory Bytes: %d / %d (%.2f%%)%s\n",
			res.BudgetUsage.MemoryBytes,
			res.BudgetUsage.MemoryLimit,
			res.BudgetUsage.MemoryUsagePercent,
			memIndicator)

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
	fmt.Printf("\n=== Comparison: %s vs %s ===\n", net1, net2)

	if res1.Status != res2.Status {
		fmt.Printf("Status Mismatch: %s (%s) vs %s (%s)\n", res1.Status, net1, res2.Status, net2)
	} else {
		fmt.Printf("Status Match: %s\n", res1.Status)
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

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", "mainnet", "Stellar network")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom RPC URL")
	debugCmd.Flags().StringVar(&rpcTokenFlag, "rpc-token", "", "RPC authentication token (can also use ERST_RPC_TOKEN env var)")
	debugCmd.Flags().BoolVar(&tracingEnabled, "tracing", false, "Enable tracing")
	debugCmd.Flags().StringVar(&otlpExporterURL, "otlp-url", "http://localhost:4318", "OTLP URL")
	debugCmd.Flags().BoolVar(&generateTrace, "generate-trace", false, "Generate trace file")
	debugCmd.Flags().StringVar(&traceOutputFile, "trace-output", "", "Trace output file")
	debugCmd.Flags().StringVar(&snapshotFlag, "snapshot", "", "Load state from JSON snapshot file")
	debugCmd.Flags().StringVar(&compareNetworkFlag, "compare-network", "", "Network to compare against (testnet, mainnet, futurenet)")
	debugCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	debugCmd.Flags().StringVar(&wasmPath, "wasm", "", "Path to local WASM file for local replay (no network required)")
	debugCmd.Flags().StringSliceVar(&args, "args", []string{}, "Mock arguments for local replay (JSON array of strings)")
	debugCmd.Flags().BoolVar(&noCacheFlag, "no-cache", false, "Disable local ledger state caching")
	debugCmd.Flags().BoolVar(&demoMode, "demo", false, "Print sample output (no network) - for testing color detection")
	debugCmd.Flags().BoolVar(&watchFlag, "watch", false, "Poll for transaction on-chain before debugging")
	debugCmd.Flags().IntVar(&watchTimeoutFlag, "watch-timeout", 30, "Timeout in seconds for watch mode")
	debugCmd.Flags().Uint32Var(&protocolVersionFlag, "protocol-version", 0, "Override protocol version for simulation (20, 21, 22, etc)")
	debugCmd.Flags().StringVar(&themeFlag, "theme", "", "Color theme (default, deuteranopia, protanopia, tritanopia, high-contrast)")

	rootCmd.AddCommand(debugCmd)
}
