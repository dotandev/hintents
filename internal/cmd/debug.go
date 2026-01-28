package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	networkFlag string
	rpcURLFlag  string
)

var debugCmd = &cobra.Command{
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
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Run simulation
		simRunner, err := simulator.NewRunner()
		if err != nil {
			return err
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

		for _, ts := range timestamps {
			if len(timestamps) > 1 {
				fmt.Printf("\n--- Simulating at Timestamp: %d ---\n", ts)
			}

			simReq := &simulator.SimulationRequest{
				EnvelopeXdr:   resp.EnvelopeXdr,
				ResultMetaXdr: resp.ResultMetaXdr,
				Timestamp:     ts,
			}

			simResp, err := simRunner.Run(simReq)
			if err != nil {
				fmt.Printf("Simulation failed at timestamp %d: %v\n", ts, err)
				continue
			}

			fmt.Printf("Status: %s\n", simResp.Status)
			if simResp.Error != "" {
				fmt.Printf("Error: %s\n", simResp.Error)
			}
			fmt.Printf("Events: %d, Logs: %d\n", len(simResp.Events), len(simResp.Logs))
		}

		return nil
	},
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")

	rootCmd.AddCommand(debugCmd)
}
