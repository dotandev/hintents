// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/dotandev/hintents/internal/session"
	"github.com/spf13/cobra"
)

var (
	sessionIDFlag string
)

// currentSessionData holds the active session context from debug command
var currentSessionData *session.SessionData

// SetCurrentSession stores the active session for later saving
func SetCurrentSession(data *session.SessionData) {
	currentSessionData = data
}

// GetCurrentSession returns the active session if any
func GetCurrentSession() *session.SessionData {
	return currentSessionData
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage debug sessions",
	Long:  `Save and resume debug sessions to preserve state across CLI invocations.`,
}

var sessionSaveCmd = &cobra.Command{
	Use:   "save [--id <session-id>]",
	Short: "Save the current debug session",
	Long: `Save the current debug session state. If no session is active, you must run
'erst debug <tx-hash>' first to create a session.

The session ID can be auto-generated or specified with --id.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Check if we have an active session
		data := GetCurrentSession()
		if data == nil {
			return fmt.Errorf("no active session to save. Run 'erst debug <tx-hash>' first")
		}

		// Generate or use provided ID
		if sessionIDFlag != "" {
			data.ID = sessionIDFlag
		} else if data.ID == "" {
			data.ID = session.GenerateID(data.TxHash)
		}

		data.Status = "saved"
		data.LastAccessAt = time.Now()

		// Open session store
		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to open session store: %w", err)
		}
		defer store.Close()

		// Run cleanup before save
		if err := store.Cleanup(ctx, session.DefaultTTL, session.DefaultMaxSessions); err != nil {
			// Log but don't fail on cleanup errors
			fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", err)
		}

		// Save session
		if err := store.Save(ctx, data); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		fmt.Printf("Session saved: %s\n", data.ID)
		fmt.Printf("  Transaction: %s\n", data.TxHash)
		fmt.Printf("  Network: %s\n", data.Network)
		fmt.Printf("  Created: %s\n", data.CreatedAt.Format(time.RFC3339))

		return nil
	},
}

var sessionResumeCmd = &cobra.Command{
	Use:   "resume <session-id>",
	Short: "Resume a saved debug session",
	Long: `Resume a previously saved debug session. This restores all transaction data,
simulation results, and context from the saved session.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		sessionID := args[0]

		// Open session store
		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to open session store: %w", err)
		}
		defer store.Close()

		// Run cleanup
		if err := store.Cleanup(ctx, session.DefaultTTL, session.DefaultMaxSessions); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", err)
		}

		// Load session
		data, err := store.Load(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}

		// Check schema version compatibility
		if data.SchemaVersion > session.SchemaVersion {
			return fmt.Errorf("session was created with a newer version of erst (schema %d > %d). Please upgrade erst", data.SchemaVersion, session.SchemaVersion)
		}

		// Update status and make it current
		data.Status = "resumed"
		SetCurrentSession(data)

		// Display session info
		fmt.Printf("Session resumed: %s\n", data.ID)
		fmt.Printf("  Transaction: %s\n", data.TxHash)
		fmt.Printf("  Network: %s\n", data.Network)
		fmt.Printf("  Created: %s\n", data.CreatedAt.Format(time.RFC3339))
		fmt.Printf("  Last accessed: %s\n", data.LastAccessAt.Format(time.RFC3339))

		// Show transaction envelope info
		if data.EnvelopeXdr != "" {
			fmt.Printf("\nTransaction Envelope:\n")
			fmt.Printf("  Size: %d bytes\n", len(data.EnvelopeXdr))
		}

		// Show simulation results if available
		if data.SimResponseJSON != "" {
			resp, err := data.ToSimulationResponse()
			if err == nil {
				fmt.Printf("\nSimulation Results:\n")
				fmt.Printf("  Status: %s\n", resp.Status)
				if resp.Error != "" {
					fmt.Printf("  Error: %s\n", resp.Error)
				}
				if len(resp.Events) > 0 {
					fmt.Printf("  Events: %d\n", len(resp.Events))
				}
				if len(resp.Logs) > 0 {
					fmt.Printf("  Logs: %d\n", len(resp.Logs))
				}
			}
		}

		return nil
	},
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved sessions",
	Long:  `List all saved debug sessions, ordered by most recently accessed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Open session store
		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to open session store: %w", err)
		}
		defer store.Close()

		// Run cleanup
		if err := store.Cleanup(ctx, session.DefaultTTL, session.DefaultMaxSessions); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cleanup failed: %v\n", err)
		}

		// List sessions
		sessions, err := store.List(ctx, 50)
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No saved sessions found.")
			return nil
		}

		fmt.Printf("Saved sessions (%d):\n\n", len(sessions))
		fmt.Printf("%-20s %-12s %-20s %-66s\n", "ID", "Network", "Last Accessed", "Transaction Hash")
		fmt.Println("--------------------------------------------------------------------------------")

		for _, s := range sessions {
			lastAccess := s.LastAccessAt.Format("2006-01-02 15:04")
			txHash := s.TxHash
			if len(txHash) > 64 {
				txHash = txHash[:64] + "..."
			}
			fmt.Printf("%-20s %-12s %-20s %-66s\n", s.ID, s.Network, lastAccess, txHash)
		}

		return nil
	},
}

var sessionDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a saved session",
	Long:  `Delete a saved debug session by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		sessionID := args[0]

		// Open session store
		store, err := session.NewStore()
		if err != nil {
			return fmt.Errorf("failed to open session store: %w", err)
		}
		defer store.Close()

		// Delete session
		if err := store.Delete(ctx, sessionID); err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}

		fmt.Printf("Session deleted: %s\n", sessionID)
		return nil
	},
}

func init() {
	sessionSaveCmd.Flags().StringVar(&sessionIDFlag, "id", "", "Custom session ID (default: auto-generated)")

	sessionCmd.AddCommand(sessionSaveCmd)
	sessionCmd.AddCommand(sessionResumeCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)

	rootCmd.AddCommand(sessionCmd)
}
