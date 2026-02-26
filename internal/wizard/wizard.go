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
	"github.com/dotandev/hintents/internal/terminal"
)

const defaultLimit = 10

type Wizard struct {
	client   *rpc.Client
	renderer terminal.Renderer
}

type SelectionResult struct {
	Hash      string
	Status    string
	CreatedAt string
}

func New(client *rpc.Client) *Wizard {
	return &Wizard{
		client:   client,
		renderer: terminal.NewANSIRenderer(),
	}
}

func (w *Wizard) WithRenderer(r terminal.Renderer) *Wizard {
	w.renderer = r
	return w
}

func (w *Wizard) SelectTransaction(ctx context.Context, account string) (*SelectionResult, error) {
	if account == "" {
		return nil, errors.WrapValidationError("account address required")
	}

	txs, err := w.client.GetAccountTransactions(ctx, account, defaultLimit)
	if err != nil {
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	if len(txs) == 0 {
		return nil, errors.WrapTransactionNotFound(fmt.Errorf("no transactions found for account"))
	}

	failed := filter(txs, isFailed)
	if len(failed) == 0 {
		w.renderer.Println("No failed transactions found. Showing all recent transactions:")
		failed = txs
	}

	selected := w.selectFromList(failed)
	if selected == nil {
		return nil, errors.WrapValidationError("transaction selection cancelled")
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

func (w *Wizard) selectFromList(txs []rpc.TransactionSummary) *SelectionResult {
	if len(txs) == 0 {
		return nil
	}

	w.displayTransactions(txs)
	choice, err := w.readUserChoice(len(txs))
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

func (w *Wizard) displayTransactions(txs []rpc.TransactionSummary) {
	w.renderer.Println("\nRecent Transactions:")
	w.renderer.Println("────────────────────────────────────────────────────")
	for i, tx := range txs {
		w.renderer.Printf("[%d] %s | %s | %s\n", i+1, tx.Status, truncateHash(tx.Hash), tx.CreatedAt)
	}
	w.renderer.Println("────────────────────────────────────────────────────")
}

func truncateHash(hash string) string {
	if len(hash) > 16 {
		return hash[:16] + "..."
	}
	return hash
}

func (w *Wizard) readUserChoice(max int) (int, error) {
	var choice int
	w.renderer.Print("Select transaction (number): ")
	_, err := w.renderer.Scanln(&choice)
	if err != nil || choice < 1 || choice > max {
		return 0, errors.WrapValidationError("invalid selection")
	}
	return choice, nil
}
