// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/db"
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
			return fmt.Errorf("Error: failed to initialize session database: %w", err)
		}

		params := db.SearchParams{
			TxHash:     searchTxFlag,
			ErrorRegex: searchErrorFlag,
			EventRegex: searchEventFlag,
			Limit:      searchLimitFlag,
		}

		sessions, err := store.SearchSessions(params)
		if err != nil {
			return fmt.Errorf("Error: search failed: %w", err)
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
