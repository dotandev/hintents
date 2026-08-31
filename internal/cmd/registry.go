// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/registry"
	"github.com/spf13/cobra"
)

// NewRegistryCommand creates the "registry" subcommand.
func NewRegistryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage Erst smart contract packages",
		Long:  `Discover, install, and manage pre-compiled Soroban smart contracts from the decentralized registry.`,
	}

	cmd.AddCommand(newRegistryInstallCommand())
	cmd.AddCommand(newRegistryInitCommand())

	return cmd
}

func newRegistryInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install [package]",
		Short: "Install a package (e.g. @openzeppelin/token@v1.0.0)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := args[0]
			ctx := context.Background()

			client, err := registry.NewClient()
			if err != nil {
				return err
			}

			path, err := client.Install(ctx, pkg)
			if err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}

			// Try to update manifest if it exists
			cwd, _ := os.Getwd()
			manifest, err := registry.LoadManifest(cwd)
			if err == nil {
				if err := manifest.AddDependency(pkg); err == nil {
					_ = registry.SaveManifest(cwd, manifest)
					logger.Logger.Info("Updated erst.json")
				}
			}

			fmt.Printf("\nSuccessfully installed %s to:\n%s\n", pkg, path)
			return nil
		},
	}
}

func newRegistryInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize an erst.json manifest in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()

			if _, err := os.Stat(cwd + "/erst.json"); err == nil {
				return fmt.Errorf("erst.json already exists")
			}

			manifest := &registry.Manifest{
				Name:         "erst-project",
				Version:      "0.1.0",
				Dependencies: make(map[string]string),
			}

			if err := registry.SaveManifest(cwd, manifest); err != nil {
				return err
			}

			fmt.Println("Initialized empty erst.json")
			return nil
		},
	}
}
