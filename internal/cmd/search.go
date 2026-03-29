// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/db"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/session"
	"github.com/spf13/cobra"
)

var (
	searchErrorFlag  string
	searchEventFlag  string
	searchTxFlag     string
	searchLimitFlag  int
	searchRecentFlag bool
)

var searchCmd = &cobra.Command{
	Use:     "search",
	GroupID: "management",
	Short:   "Search through saved debugging sessions",
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
		if searchRecentFlag {
			uiStore, err := session.NewUIStateStore()
			if err != nil {
				return fmt.Errorf("failed to open viewer state: %w", err)
			}
			defer uiStore.Close()
			queries, err := uiStore.RecentSearches(cmd.Context(), 10)
			if err != nil {
				return fmt.Errorf("failed to load recent searches: %w", err)
			}
			if len(queries) == 0 {
				fmt.Println("No recent searches.")
				return nil
			}
			fmt.Printf("Recent searches (%d):\n", len(queries))
			for i, q := range queries {
				fmt.Printf("  %d. %s\n", i+1, q)
			}
			return nil
		}

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

		// Persist non-empty search terms for future recall (best-effort).
		if uiStore, err := session.NewUIStateStore(); err == nil {
			defer uiStore.Close()
			for _, q := range []string{searchErrorFlag, searchEventFlag, searchTxFlag} {
				if q != "" {
					_ = uiStore.AppendRecentSearch(cmd.Context(), q)
				}
			}
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchErrorFlag, "error", "", "Regex pattern to match error messages")
	searchCmd.Flags().StringVar(&searchEventFlag, "event", "", "Regex pattern to match events")
	searchCmd.Flags().StringVar(&searchTxFlag, "tx", "", "Transaction hash to search for")
	searchCmd.Flags().IntVar(&searchLimitFlag, "limit", 10, "Maximum number of results to return")
	searchCmd.Flags().BoolVar(&searchRecentFlag, "recent", false, "Show recent search queries")

	rootCmd.AddCommand(searchCmd)
}
