// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package wizard

import (
	"context"
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/dotandev/hintents/internal/rpc"
)

const defaultLimit = 10

type Wizard struct {
	client *rpc.Client
}

type SelectionResult struct {
	Hash      string
	Status    string
	CreatedAt string
}

func New(client *rpc.Client) *Wizard {
	return &Wizard{client: client}
}

func (w *Wizard) SelectTransaction(ctx context.Context, account string) (*SelectionResult, error) {
	if account == "" {
		return nil, fmt.Errorf("%w: account address required", errors.ErrInvalidNetwork)
	}

	txs, err := w.client.GetAccountTransactions(ctx, account, defaultLimit)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", errors.ErrRPCConnectionFailed, err)
	}

	if len(txs) == 0 {
		return nil, fmt.Errorf("%w: no transactions found for account", errors.ErrTransactionNotFound)
	}

	failed := filter(txs, isFailed)
	if len(failed) == 0 {
		fmt.Println("No failed transactions found. Showing all recent transactions:")
		failed = txs
	}

	selected := selectFromList(failed)
	if selected == nil {
		return nil, fmt.Errorf("transaction selection cancelled")
	}

	logger.Logger.Info("Transaction selected", "hash", selected.Hash, "status", selected.Status)
	return selected, nil
}

func isFailed(tx rpc.TransactionSummary) bool {
	return strings.Contains(tx.Status, "failed")
}

func filter(txs []rpc.TransactionSummary, predicate func(rpc.TransactionSummary) bool) []rpc.TransactionSummary {
	result := make([]rpc.TransactionSummary, 0, len(txs))
	for _, tx := range txs {
		if predicate(tx) {
			result = append(result, tx)
		}
	}
	return result
}

func selectFromList(txs []rpc.TransactionSummary) *SelectionResult {
	if len(txs) == 0 {
		return nil
	}

	displayTransactions(txs)
	choice, err := readUserChoice(len(txs))
	if err != nil {
		return nil
	}

	selected := txs[choice-1]
	return &SelectionResult{
		Hash:      selected.Hash,
		Status:    selected.Status,
		CreatedAt: selected.CreatedAt,
	}
}

func displayTransactions(txs []rpc.TransactionSummary) {
	fmt.Println("\nRecent Transactions:")
	fmt.Println("────────────────────────────────────────────────────")
	for i, tx := range txs {
		fmt.Printf("[%d] %s | %s | %s\n", i+1, tx.Status, truncateHash(tx.Hash), tx.CreatedAt)
	}
	fmt.Println("────────────────────────────────────────────────────")
}

func truncateHash(hash string) string {
	if len(hash) > 16 {
		return hash[:16] + "..."
	}
	return hash
}

func readUserChoice(max int) (int, error) {
	var choice int
	fmt.Print("Select transaction (number): ")
	_, err := fmt.Scanln(&choice)
	if err != nil || choice < 1 || choice > max {
		return 0, fmt.Errorf("invalid selection")
	}
	return choice, nil
}
