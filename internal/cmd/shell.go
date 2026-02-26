// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/shell"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	shellNetworkFlag string
	shellRPCURLFlag  string
	shellRPCToken    string
	shellInitState   string
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Start an interactive shell for contract invocations",
	Long: `Start a persistent interactive shell where you can invoke multiple contracts
consecutively without losing the local ledger state between commands.

The shell maintains a stateful ledger that persists across invocations, allowing
you to test complex multi-step contract interactions.

Examples:
  erst shell                                    Start shell with empty state
  erst shell --network testnet                  Start shell on testnet
  erst shell --init-state snapshot.json         Start with initial state
  
Shell Commands:
  invoke <contract-id> <function> [args...]    Invoke a contract function
  state                                         Show current ledger state
  state save <file>                             Save current state to file
  state load <file>                             Load state from file
  state reset                                   Reset to initial state
  help                                          Show available commands
  exit                                          Exit the shell`,
	Args: cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate network flag
		switch rpc.Network(shellNetworkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			return nil
		default:
			return errors.WrapInvalidNetwork(shellNetworkFlag)
		}
	},
	RunE: runShell,
}

func init() {
	shellCmd.Flags().StringVarP(&shellNetworkFlag, "network", "n", string(rpc.Testnet), "Stellar network to use (testnet, mainnet, futurenet)")
	shellCmd.Flags().StringVar(&shellRPCURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")
	shellCmd.Flags().StringVar(&shellRPCToken, "rpc-token", "", "RPC authentication token")
	shellCmd.Flags().StringVar(&shellInitState, "init-state", "", "Initial ledger state file (JSON)")

	rootCmd.AddCommand(shellCmd)
}

func runShell(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize RPC client
	var rpcClient *rpc.Client
	if shellRPCURLFlag != "" {
		rpcClient = rpc.NewClientWithURL(shellRPCURLFlag, rpc.Network(shellNetworkFlag))
	} else {
		rpcClient = rpc.NewClient(rpc.Network(shellNetworkFlag))
	}

	// Initialize simulator runner
	runner, err := simulator.NewRunner("", false)
	if err != nil {
		return errors.WrapSimulatorNotFound(err.Error())
	}

	// Create shell session
	session := shell.NewSession(runner, rpcClient, rpc.Network(shellNetworkFlag))

	// Load initial state if provided
	if shellInitState != "" {
		if err := session.LoadState(shellInitState); err != nil {
			logger.Logger.Warn("Failed to load initial state", "error", err)
			fmt.Printf("Warning: Could not load initial state from %s: %v\n", shellInitState, err)
		} else {
			fmt.Printf("Loaded initial state from %s\n", shellInitState)
		}
	}

	// Print welcome message
	printWelcome()

	// Start REPL
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("erst> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse and execute command
		if err := executeShellCommand(ctx, session, line); err != nil {
			if err.Error() == "exit" {
				break
			}
			fmt.Printf("Error: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	fmt.Println("\nGoodbye!")
	return nil
}

func printWelcome() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  Erst Interactive Shell                                       ║")
	fmt.Println("║  Persistent ledger state for multi-step contract testing     ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Type 'help' for available commands or 'exit' to quit.")
	fmt.Println()
}

func executeShellCommand(ctx context.Context, session *shell.Session, line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "help", "?":
		printHelp()
		return nil

	case "exit", "quit":
		return fmt.Errorf("exit")

	case "invoke":
		return handleInvoke(ctx, session, args)

	case "state":
		return handleState(session, args)

	case "clear":
		fmt.Print("\033[H\033[2J")
		return nil

	default:
		return fmt.Errorf("unknown command: %s (type 'help' for available commands)", command)
	}
}

func printHelp() {
	fmt.Println("Available commands:")
	fmt.Println()
	fmt.Println("  invoke <contract-id> <function> [args...]")
	fmt.Println("      Invoke a contract function with the given arguments")
	fmt.Println("      Example: invoke CAAAA... transfer alice bob 100")
	fmt.Println()
	fmt.Println("  state")
	fmt.Println("      Display current ledger state summary")
	fmt.Println()
	fmt.Println("  state save <file>")
	fmt.Println("      Save current ledger state to a JSON file")
	fmt.Println("      Example: state save my-state.json")
	fmt.Println()
	fmt.Println("  state load <file>")
	fmt.Println("      Load ledger state from a JSON file")
	fmt.Println("      Example: state load my-state.json")
	fmt.Println()
	fmt.Println("  state reset")
	fmt.Println("      Reset ledger state to initial state")
	fmt.Println()
	fmt.Println("  clear")
	fmt.Println("      Clear the terminal screen")
	fmt.Println()
	fmt.Println("  help, ?")
	fmt.Println("      Show this help message")
	fmt.Println()
	fmt.Println("  exit, quit")
	fmt.Println("      Exit the shell")
	fmt.Println()
}

func handleInvoke(ctx context.Context, session *shell.Session, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: invoke <contract-id> <function> [args...]")
	}

	contractID := args[0]
	function := args[1]
	funcArgs := args[2:]

	fmt.Printf("Invoking %s.%s(%s)...\n", contractID, function, strings.Join(funcArgs, ", "))

	result, err := session.Invoke(ctx, contractID, function, funcArgs)
	if err != nil {
		return fmt.Errorf("invocation failed: %w", err)
	}

	// Display result
	fmt.Println()
	fmt.Println("Result:")
	fmt.Printf("  Status: %s\n", result.Status)
	if result.Error != "" {
		fmt.Printf("  Error: %s\n", result.Error)
	}
	if len(result.Events) > 0 {
		fmt.Printf("  Events: %d\n", len(result.Events))
		for i, event := range result.Events {
			fmt.Printf("    [%d] %s\n", i, event)
		}
	}
	if len(result.Logs) > 0 {
		fmt.Printf("  Logs: %d\n", len(result.Logs))
		for i, log := range result.Logs {
			fmt.Printf("    [%d] %s\n", i, log)
		}
	}
	fmt.Println()

	return nil
}

func handleState(session *shell.Session, args []string) error {
	if len(args) == 0 {
		// Display state summary
		summary := session.GetStateSummary()
		fmt.Println()
		fmt.Println("Current Ledger State:")
		fmt.Printf("  Entries: %d\n", summary.EntryCount)
		fmt.Printf("  Sequence: %d\n", summary.LedgerSequence)
		fmt.Printf("  Timestamp: %d\n", summary.Timestamp)
		fmt.Printf("  Invocations: %d\n", summary.InvocationCount)
		fmt.Println()
		return nil
	}

	subcommand := args[0]
	switch subcommand {
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("usage: state save <file>")
		}
		filename := args[1]
		if err := session.SaveState(filename); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
		fmt.Printf("State saved to %s\n", filename)
		return nil

	case "load":
		if len(args) < 2 {
			return fmt.Errorf("usage: state load <file>")
		}
		filename := args[1]
		if err := session.LoadState(filename); err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}
		fmt.Printf("State loaded from %s\n", filename)
		return nil

	case "reset":
		session.ResetState()
		fmt.Println("State reset to initial state")
		return nil

	default:
		return fmt.Errorf("unknown state subcommand: %s", subcommand)
	}
}
