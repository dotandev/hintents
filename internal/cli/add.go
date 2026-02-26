// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"

	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/spf13/cobra"
)

// AddCommand implements the 'add' operation for managing resources
type AddCommand struct {
	BaseCommand
	// ConfigManager is not strictly needed as we use package-level functions in config
}

// NewAddCommand creates a new add command instance
func NewAddCommand() *AddCommand {
	return &AddCommand{
		BaseCommand: BaseCommand{
			Use:             "add [resource] [flags]",
			Short:           "Add a new resource to the configuration",
			Long:            "Add a new resource (e.g., network, identity, contract) to the local configuration.",
			Example:         "  erst add network --name testnet --rpc https://horizon-testnet.stellar.org",
			ProtocolVersion: 2, // V2 CLI
		},
	}
}

func (c *AddCommand) Name() string {
	return "add"
}

func (c *AddCommand) Description() string {
	return c.Short
}

func (c *AddCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     c.Use,
		Short:   c.Short,
		Long:    c.Long,
		Example: c.Example,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
	}

	// Add subcommands
	cmd.AddCommand(NewAddNetworkCommand().CreateCobraCommand())

	return cmd
}

func (c *AddCommand) Validate(ctx context.Context, args []string) error {
	return nil
}

func (c *AddCommand) Execute(ctx context.Context, args []string) error {
	return nil
}

// AddNetworkCommand implements 'add network'
type AddNetworkCommand struct {
	BaseCommand

	// Flags
	Name       string
	RPCURL     string
	Passphrase string
}

func NewAddNetworkCommand() *AddNetworkCommand {
	return &AddNetworkCommand{
		BaseCommand: BaseCommand{
			Use:   "network",
			Short: "Add a new network configuration",
			Long:  "Add a new Stellar network configuration to the local registry.",
		},
	}
}

func (c *AddNetworkCommand) Name() string {
	return "network"
}

func (c *AddNetworkCommand) Description() string {
	return c.Short
}

func (c *AddNetworkCommand) CreateCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   c.Use,
		Short: c.Short,
		Long:  c.Long,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.Execute(cmd.Context(), args)
		},
	}

	cmd.Flags().StringVar(&c.Name, "name", "", "Network name (required)")
	cmd.Flags().StringVar(&c.RPCURL, "rpc-url", "", "RPC URL (required)")
	cmd.Flags().StringVar(&c.Passphrase, "passphrase", "", "Network passphrase (optional)")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("rpc-url")

	return cmd
}

func (c *AddNetworkCommand) Validate(ctx context.Context, args []string) error {
	if c.Name == "" {
		return fmt.Errorf("network name is required")
	}
	if c.RPCURL == "" {
		return fmt.Errorf("rpc-url is required")
	}
	return nil
}

func (c *AddNetworkCommand) Execute(ctx context.Context, args []string) error {
	fmt.Printf("Adding network configuration: %s\n", c.Name)
	fmt.Printf("RPC URL: %s\n", c.RPCURL)

	netConfig := rpc.NetworkConfig{
		Name:              c.Name,
		HorizonURL:        c.RPCURL,
		NetworkPassphrase: c.Passphrase,
		SorobanRPCURL:     c.RPCURL, // Assuming same URL for now, or could add flag
	}

	if err := config.AddCustomNetwork(c.Name, netConfig); err != nil {
		return fmt.Errorf("failed to save network config: %w", err)
	}

	fmt.Println("Network configuration saved successfully.")
	return nil
}
