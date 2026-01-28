package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/testgen"
	"github.com/spf13/cobra"
)

var (
	genTestLang      string
	genTestOutput    string
	genTestName      string
	genTestRPCToken  string
	genTestNetwork   string
	genTestRPCURL    string
)

var generateTestCmd = &cobra.Command{
	Use:   "generate-test <transaction-hash>",
	Short: "Generate regression tests from a transaction",
	Long: `Generate regression tests from a recorded transaction trace.
This creates test files that can be used to ensure bugs don't reoccur.

The command fetches the transaction data from the network and generates
test files in Go and/or Rust that replay the transaction.

Example:
  erst generate-test 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
  erst generate-test --lang go --name my_test <tx-hash>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

		// Create RPC client
		var client *rpc.Client
		if genTestRPCURL != "" {
			client = rpc.NewClientWithURL(genTestRPCURL, rpc.Network(genTestNetwork), genTestRPCToken)
		} else {
			client = rpc.NewClient(rpc.Network(genTestNetwork), genTestRPCToken)
		}

		// Get current working directory as default output
		if genTestOutput == "" {
			genTestOutput = "."
		}

		// Create test generator
		generator := testgen.NewTestGenerator(client, genTestOutput)

		// Generate tests
		fmt.Printf("Generating %s regression test(s) for transaction: %s\n", genTestLang, txHash)
		if err := generator.GenerateTests(cmd.Context(), txHash, genTestLang, genTestName); err != nil {
			return fmt.Errorf("failed to generate tests: %w", err)
		}

		fmt.Println("✓ Test generation completed successfully")
		return nil
	},
}

func init() {
	generateTestCmd.Flags().StringVarP(&genTestLang, "lang", "l", "both", "Target language (go, rust, or both)")
	generateTestCmd.Flags().StringVarP(&genTestOutput, "output", "o", "", "Output directory (defaults to current directory)")
	generateTestCmd.Flags().StringVarP(&genTestName, "name", "", "", "Custom test name (defaults to transaction hash)")
	generateTestCmd.Flags().StringVarP(&genTestNetwork, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	generateTestCmd.Flags().StringVar(&genTestRPCURL, "rpc-url", "", "Custom Horizon RPC URL to use")
	generateTestCmd.Flags().StringVar(&genTestRPCToken, "rpc-token", "", "RPC authentication token (can also use ERST_RPC_TOKEN env var)")

	rootCmd.AddCommand(generateTestCmd)
}
