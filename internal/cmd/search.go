// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/db"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/spf13/cobra"
)

var (
	searchErrorFlag string
	searchEventFlag string
	searchTxFlag    string
	searchLimitFlag int
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search through saved debugging sessions",
	Long: `Search through the history of debugging sessions to find past transactions,
errors, or events. Supports regex patterns for flexible matching.

You can search by:
  • Transaction hash (exact match)
  • Error message patterns (regex)
  • Event patterns (regex)
  • Combine multiple filters

Results are ordered by timestamp (most recent first) and limited by --limit flag.`,
	Example: `  # Search for specific transaction
  erst search --tx abc123...def789

  # Find sessions with specific error patterns
  erst search --error "insufficient balance"

  # Search for contract events
  erst search --event "transfer|mint"

  # Combine filters and limit results
  erst search --error "panic" --limit 5`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := db.InitDB()
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("failed to initialize session database: %v", err))
		}

		params := db.SearchParams{
			TxHash:     searchTxFlag,
			ErrorRegex: searchErrorFlag,
			EventRegex: searchEventFlag,
			Limit:      searchLimitFlag,
		}

		sessions, err := store.SearchSessions(params)
		if err != nil {
			return errors.WrapValidationError(fmt.Sprintf("search failed: %v", err))
		}

		if len(sessions) == 0 {
			fmt.Println("No matching sessions found.")
			return nil
		}

		fmt.Printf("Found %d matching sessions:\n", len(sessions))
		for _, s := range sessions {
			fmt.Println("--------------------------------------------------")
			fmt.Printf("ID: %d\n", s.ID)
			fmt.Printf("Time: %s\n", s.Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("Tx Hash: %s\n", s.TxHash)
			fmt.Printf("Network: %s\n", s.Network)
			fmt.Printf("Status: %s\n", s.Status)
			if s.ErrorMsg != "" {
				fmt.Printf("Error: %s\n", s.ErrorMsg)
			}
			if len(s.Events) > 0 {
				fmt.Println("Events:")
				for _, e := range s.Events {
					fmt.Printf("  - %s\n", e)
				}
			}
		}
		fmt.Println("--------------------------------------------------")

		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchErrorFlag, "error", "", "Regex pattern to match error messages")
	searchCmd.Flags().StringVar(&searchEventFlag, "event", "", "Regex pattern to match events")
	searchCmd.Flags().StringVar(&searchTxFlag, "tx", "", "Transaction hash to search for")
	searchCmd.Flags().IntVar(&searchLimitFlag, "limit", 10, "Maximum number of results to return")

	rootCmd.AddCommand(searchCmd)
}
