// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/logger"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/protocols/horizon/effects"
)

type TransactionSummary struct {
	Hash      string
	Status    string
	CreatedAt string
}

type AccountSummary struct {
	ID            string
	Sequence      int64
	SubentryCount int32
}

type EventSummary struct {
	ID   string
	Type string
}

func (c *Client) GetAccountTransactions(ctx context.Context, account string, limit int) ([]TransactionSummary, error) {
	logger.Logger.Debug("Fetching account transactions", "account", account)

	pageSize := normalizePageSize(limit)
	req := horizonclient.TransactionRequest{
		ForAccount: account,
		Limit:      uint(pageSize),
		Order:      horizonclient.OrderDesc,
	}

	transactions, err := pageIterator[hProtocol.TransactionsPage, hProtocol.Transaction]{
		first: func() (hProtocol.TransactionsPage, error) {
			return c.Horizon.Transactions(req)
		},
		next: func(page hProtocol.TransactionsPage) (hProtocol.TransactionsPage, error) {
			return c.Horizon.NextTransactionsPage(page)
		},
		records: func(page hProtocol.TransactionsPage) []hProtocol.Transaction {
			return page.Embedded.Records
		},
		max: limit,
	}.collect()
	if err != nil {
		logger.Logger.Error("Failed to fetch account transactions", "account", account, "error", err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	summaries := make([]TransactionSummary, 0, len(transactions))
	for _, tx := range transactions {
		summaries = append(summaries, TransactionSummary{
			Hash:      tx.Hash,
			Status:    getTransactionStatus(tx),
			CreatedAt: tx.LedgerCloseTime.Format("2006-01-02 15:04:05"),
		})
	}

	logger.Logger.Debug("Account transactions retrieved", "count", len(summaries))
	return summaries, nil
}

// GetEventsForAccount fetches effects (treated as events) for an account using shared page iteration.
func (c *Client) GetEventsForAccount(ctx context.Context, account string, limit int) ([]EventSummary, error) {
	logger.Logger.Debug("Fetching account events", "account", account)

	pageSize := normalizePageSize(limit)
	req := horizonclient.EffectRequest{
		ForAccount: account,
		Limit:      uint(pageSize),
		Order:      horizonclient.OrderDesc,
	}

	eventRecords, err := pageIterator[effects.EffectsPage, effects.Effect]{
		first: func() (effects.EffectsPage, error) {
			return c.Horizon.Effects(req)
		},
		next: func(page effects.EffectsPage) (effects.EffectsPage, error) {
			return c.Horizon.NextEffectsPage(page)
		},
		records: func(page effects.EffectsPage) []effects.Effect {
			return page.Embedded.Records
		},
		max: limit,
	}.collect()
	if err != nil {
		logger.Logger.Error("Failed to fetch account events", "account", account, "error", err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	out := make([]EventSummary, 0, len(eventRecords))
	for _, evt := range eventRecords {
		out = append(out, EventSummary{
			ID:   evt.GetID(),
			Type: evt.GetType(),
		})
	}

	logger.Logger.Debug("Account events retrieved", "count", len(out))
	return out, nil
}

// GetAccounts fetches account records using shared page iteration.
func (c *Client) GetAccounts(ctx context.Context, limit int) ([]AccountSummary, error) {
	logger.Logger.Debug("Fetching accounts")

	pageSize := normalizePageSize(limit)
	req := horizonclient.AccountsRequest{
		Limit: uint(pageSize),
		Order: horizonclient.OrderDesc,
	}

	accountRecords, err := pageIterator[hProtocol.AccountsPage, hProtocol.Account]{
		first: func() (hProtocol.AccountsPage, error) {
			return c.Horizon.Accounts(req)
		},
		next: func(page hProtocol.AccountsPage) (hProtocol.AccountsPage, error) {
			return c.Horizon.NextAccountsPage(page)
		},
		records: func(page hProtocol.AccountsPage) []hProtocol.Account {
			return page.Embedded.Records
		},
		max: limit,
	}.collect()
	if err != nil {
		logger.Logger.Error("Failed to fetch accounts", "error", err)
		return nil, errors.WrapRPCConnectionFailed(err)
	}

	out := make([]AccountSummary, 0, len(accountRecords))
	for _, acc := range accountRecords {
		out = append(out, AccountSummary{
			ID:            acc.AccountID,
			Sequence:      acc.Sequence,
			SubentryCount: acc.SubentryCount,
		})
	}

	logger.Logger.Debug("Accounts retrieved", "count", len(out))
	return out, nil
}

func getTransactionStatus(tx hProtocol.Transaction) string {
	if tx.Successful {
		return "success"
	}
	return "failed"
}
