// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dotandev/hintents/internal/daemon"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/telemetry"
	"github.com/spf13/cobra"
)

var (
	daemonPort      string
	daemonNetwork   string
	daemonRPCURL    string
	daemonAuthToken string
	daemonTracing   bool
	daemonOTLPURL   string
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start JSON-RPC server for remote debugging",
	Long: `Start a JSON-RPC 2.0 server that exposes ERST functionality for remote tools and IDEs.

Endpoints:
  - debug_transaction: Debug a failed transaction
  - get_trace: Get execution traces for a transaction

Example:
  erst daemon --port 8080 --network testnet
  erst daemon --port 8080 --auth-token secret123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Initialize OpenTelemetry if enabled
		var cleanup func()
		if daemonTracing {
			var err error
			cleanup, err = telemetry.Init(ctx, telemetry.Config{
				Enabled:     true,
				ExporterURL: daemonOTLPURL,
				ServiceName: "erst-daemon",
			})
			if err != nil {
				return errors.WrapValidationError(fmt.Sprintf("failed to initialize telemetry: %v", err))
			}
			defer cleanup()
		}

		// Validate network
		switch rpc.Network(daemonNetwork) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
		default:
			return errors.WrapInvalidNetwork(daemonNetwork)
		}

		// Create server
		server, err := daemon.NewServer(daemon.Config{
			Port:      daemonPort,
			Network:   daemonNetwork,
			RPCURL:    daemonRPCURL,
			AuthToken: daemonAuthToken,
		})
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to create server: %v", err))
		}

		// Setup graceful shutdown
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Handle interrupt signals
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			fmt.Println("\nReceived interrupt signal, shutting down...")
			cancel()
		}()

		fmt.Printf("Starting ERST daemon on port %s\n", daemonPort)
		fmt.Printf("Network: %s\n", daemonNetwork)
		if daemonRPCURL != "" {
			fmt.Printf("RPC URL: %s\n", daemonRPCURL)
		}
		if daemonAuthToken != "" {
			fmt.Println("Authentication: enabled")
		}

		// Start server
		return server.Start(ctx, daemonPort)
	},
}

func init() {
	daemonCmd.Flags().StringVarP(&daemonPort, "port", "p", "8080", "Port to listen on")
	daemonCmd.Flags().StringVarP(&daemonNetwork, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	daemonCmd.Flags().StringVar(&daemonRPCURL, "rpc-url", "", "Custom Horizon RPC URL to use")
	daemonCmd.Flags().StringVar(&daemonAuthToken, "auth-token", "", "Authentication token for API access")
	daemonCmd.Flags().BoolVar(&daemonTracing, "tracing", false, "Enable OpenTelemetry tracing")
	daemonCmd.Flags().StringVar(&daemonOTLPURL, "otlp-url", "http://localhost:4318", "OTLP exporter URL")

	rootCmd.AddCommand(daemonCmd)
}
