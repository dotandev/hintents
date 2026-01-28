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
		return nil
	},
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")

	rootCmd.AddCommand(debugCmd)
}
